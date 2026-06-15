"""Mapbox Directions + Geocoding client cho in-app walking navigation.

Phục vụ 3 tool Gemini:
  - start_walking_navigation: geocode destination + lấy route, lưu vào session state
  - announce_navigation_step: lấy step hiện tại + check deviation, trả về cho Gemini narrate
  - stop_navigation: kết thúc + cleanup

Mapbox Directions: https://docs.mapbox.com/api/navigation/directions/
Mapbox Geocoding v5: https://docs.mapbox.com/api/search/geocoding/
Profile: mapbox/walking — turn-by-turn, ưu tiên vỉa hè, đường nhỏ, công viên.
Free tier: 100,000 requests/tháng.

State lưu trong ToolRouter.user_context["navigation_state"] (per-session, in-memory).
Việc persist xuống DB là bước sau (Bước 7-8 trong plan).
"""

from __future__ import annotations

import logging
import math
import re
import time
from dataclasses import dataclass
from typing import Any
from urllib.parse import quote

import httpx

logger = logging.getLogger("aisee-agent")


# ── Data classes ──────────────────────────────────────────────

@dataclass(frozen=True, slots=True)
class GeoPoint:
    lat: float
    lng: float

    def to_tuple(self) -> tuple[float, float]:
        return (self.lng, self.lat)  # Mapbox expects lng,lat order


@dataclass(slots=True)
class NavigationStep:
    """Một turn instruction từ Mapbox."""
    index: int
    distance_m: float
    duration_s: float
    name: str
    maneuver: str
    modifier: str | None
    instruction_text: str  # Câu hướng dẫn đầy đủ (English — Gemini sẽ dịch)
    voice_text: str  # Câu ngắn để TTS đọc
    location: GeoPoint


@dataclass(slots=True)
class NavigationRoute:
    steps: list[NavigationStep]
    geometry: list[GeoPoint]
    total_distance_m: float
    total_duration_s: float
    destination_name: str
    destination_point: GeoPoint


@dataclass(slots=True)
class NavigationSession:
    """State cho 1 cuốc navigation đang active."""
    route: NavigationRoute
    started_at: float
    destination_query: str
    origin_point: GeoPoint
    current_step_index: int = 0
    last_deviation_alert_at: float = 0.0
    last_announced_step: int = -1
    deviation_count: int = 0
    outcome: str = "in_progress"


# ── Geo helpers ──────────────────────────────────────────────

def haversine_m(p1: GeoPoint, p2: GeoPoint) -> float:
    """Khoảng cách 2 điểm trên mặt cầu (mét)."""
    r = 6371000.0
    lat1, lat2 = math.radians(p1.lat), math.radians(p2.lat)
    dlat = lat2 - lat1
    dlng = math.radians(p2.lng - p1.lng)
    a = math.sin(dlat / 2) ** 2 + math.cos(lat1) * math.cos(lat2) * math.sin(dlng / 2) ** 2
    return 2 * r * math.asin(math.sqrt(a))


def point_to_polyline_distance_m(p: GeoPoint, polyline: list[GeoPoint]) -> float:
    """Khoảng cách tối thiểu từ p tới polyline (mét). Approx: dùng haversine tới segment midpoint."""
    if not polyline:
        return float("inf")
    if len(polyline) == 1:
        return haversine_m(p, polyline[0])
    min_dist = float("inf")
    for i in range(len(polyline) - 1):
        mid_lat = (polyline[i].lat + polyline[i + 1].lat) / 2
        mid_lng = (polyline[i].lng + polyline[i + 1].lng) / 2
        d = haversine_m(p, GeoPoint(mid_lat, mid_lng))
        if d < min_dist:
            min_dist = d
    return min_dist


# ── Main client ──────────────────────────────────────────────

class MapboxNavigationClient:
    """Wrap Mapbox Directions (walking) + Geocoding v5."""

    def __init__(
        self,
        token: str,
        directions_base_url: str = "https://api.mapbox.com/directions/v5",
        geocoding_base_url: str = "https://api.mapbox.com/geocoding/v5",
        timeout: float = 12.0,
        transport: httpx.AsyncBaseTransport | None = None,
    ) -> None:
        self._token = token
        self._directions_url = directions_base_url.rstrip("/")
        self._geocoding_url = geocoding_base_url.rstrip("/")
        self._timeout = timeout
        self._transport = transport

    @classmethod
    def from_env(cls) -> "MapboxNavigationClient":
        from config import (
            MAPBOX_NAVIGATION_TOKEN,
            MAPBOX_DIRECTIONS_BASE_URL,
            MAPBOX_GEOCODING_BASE_URL,
        )
        return cls(
            token=MAPBOX_NAVIGATION_TOKEN,
            directions_base_url=MAPBOX_DIRECTIONS_BASE_URL,
            geocoding_base_url=MAPBOX_GEOCODING_BASE_URL,
        )

    @property
    def is_configured(self) -> bool:
        return bool(self._token and self._token.strip())

    async def geocode(self, query: str, *, proximity: GeoPoint | None = None) -> dict[str, Any]:
        query = (query or "").strip()
        if not query:
            return {"status": "error", "error": "geocode: empty query"}
        if not self.is_configured:
            return {"status": "error", "error": "MAPBOX_NAVIGATION_TOKEN is not configured"}

        url = f"{self._geocoding_url}/mapbox.places/{quote(query, safe='')}.json"
        params: dict[str, Any] = {
            "limit": 1,
            "language": "vi",
            "access_token": self._token,
        }
        if proximity is not None:
            params["proximity"] = f"{proximity.lng},{proximity.lat}"

        try:
            async with httpx.AsyncClient(timeout=self._timeout, transport=self._transport) as client:
                response = await client.get(url, params=params)
                response.raise_for_status()
        except httpx.HTTPError as exc:
            logger.warning("mapbox geocode http error: %s", exc)
            return {"status": "error", "error": f"geocode network error: {exc}"}

        data = response.json()
        features = data.get("features") or []
        if not features:
            return {"status": "error", "error": f"không tìm thấy địa điểm: {query}"}

        f = features[0]
        center = f.get("center") or []
        if len(center) < 2:
            return {"status": "error", "error": "geocode response missing center"}
        return {
            "status": "ok",
            "name": str(f.get("place_name") or f.get("text") or query),
            "short_name": str(f.get("text") or query),
            "lat": float(center[1]),
            "lng": float(center[0]),
        }

    async def walking_route(
        self,
        origin: GeoPoint,
        destination: GeoPoint,
    ) -> dict[str, Any]:
        if not self.is_configured:
            return {"status": "error", "error": "MAPBOX_NAVIGATION_TOKEN is not configured"}

        coords = f"{origin.lng},{origin.lat};{destination.lng},{destination.lat}"
        url = f"{self._directions_url}/mapbox/walking/{coords}"
        params = {
            "geometries": "geojson",
            "steps": "true",
            "voice_instructions": "true",
            "banner_instructions": "false",
            "language": "vi",
            "overview": "full",
            "access_token": self._token,
        }
        try:
            async with httpx.AsyncClient(timeout=self._timeout, transport=self._transport) as client:
                response = await client.get(url, params=params)
                response.raise_for_status()
        except httpx.HTTPError as exc:
            logger.warning("mapbox directions http error: %s", exc)
            return {"status": "error", "error": f"directions network error: {exc}"}

        data = response.json()
        routes = data.get("routes") or []
        if not routes:
            err = data.get("message") or data.get("code") or "no route found"
            return {"status": "error", "error": f"directions: {err}"}

        route = routes[0]
        return {
            "status": "ok",
            "route": self._parse_route(route, destination),
        }

    def _parse_route(self, raw: dict[str, Any], destination_point: GeoPoint) -> NavigationRoute:
        geometry = raw.get("geometry") or {}
        coords = geometry.get("coordinates") or []
        polyline = [GeoPoint(lat=p[1], lng=p[0]) for p in coords if len(p) >= 2]

        legs = raw.get("legs") or []
        steps: list[NavigationStep] = []
        idx = 0
        for leg in legs:
            for s in (leg.get("steps") or []):
                maneuver = s.get("maneuver") or {}
                step_loc = maneuver.get("location") or [0, 0]
                voice = s.get("voiceInstructions") or []
                voice_text = ""
                if voice:
                    raw_text = voice[0].get("ssmlAnnouncement") or voice[0].get("announcement") or ""
                    voice_text = _strip_ssml(raw_text)
                fallback = voice_text or s.get("name") or _maneuver_to_text(maneuver)

                steps.append(NavigationStep(
                    index=idx,
                    distance_m=float(s.get("distance") or 0),
                    duration_s=float(s.get("duration") or 0),
                    name=str(s.get("name") or ""),
                    maneuver=str(maneuver.get("type") or "continue"),
                    modifier=maneuver.get("modifier"),
                    instruction_text=fallback,
                    voice_text=fallback,
                    location=GeoPoint(
                        lat=float(step_loc[1]) if len(step_loc) >= 2 else 0,
                        lng=float(step_loc[0]) if len(step_loc) >= 1 else 0,
                    ),
                ))
                idx += 1

        return NavigationRoute(
            steps=steps,
            geometry=polyline,
            total_distance_m=float(raw.get("distance") or 0),
            total_duration_s=float(raw.get("duration") or 0),
            destination_name=str((raw.get("destination") or {}).get("name") or ""),
            destination_point=destination_point,
        )


# ── Pure helpers (testable) ─────────────────────────────────

def _strip_ssml(text: str) -> str:
    return re.sub(r"<[^>]+>", "", text).strip()


def _maneuver_to_text(maneuver: dict[str, Any]) -> str:
    mtype = maneuver.get("type") or "continue"
    modifier = maneuver.get("modifier") or ""
    if mtype == "depart":
        return "Depart and head toward the destination"
    if mtype == "arrive":
        return "You have arrived at your destination"
    if mtype == "turn":
        return f"Turn {modifier}".strip()
    if mtype == "new name":
        return "Continue on the new road"
    if mtype == "continue":
        return "Continue straight"
    if mtype == "merge":
        return f"Merge {modifier}".strip()
    if mtype == "fork":
        return f"Keep {modifier} at the fork".strip()
    if mtype == "roundabout" or mtype == "rotary":
        return "Enter the roundabout"
    return "Continue"


def evaluate_progress(
    session: NavigationSession,
    current: GeoPoint,
    *,
    deviation_threshold_m: float,
    deviation_cooldown_s: float,
    now: float | None = None,
) -> dict[str, Any]:
    now = now or time.time()
    route = session.route

    # 1. Tìm step hiện tại — step gần current nhất
    best_idx = session.current_step_index
    best_dist = float("inf")
    for s in route.steps:
        d = haversine_m(current, s.location)
        if d < best_dist:
            best_dist = d
            best_idx = s.index

    # 2. Tính deviation so với polyline
    deviation_m = point_to_polyline_distance_m(current, route.geometry)
    is_deviated = deviation_m > deviation_threshold_m

    # 3. Cooldown tránh alert liên tục
    should_alert = is_deviated and (
        now - session.last_deviation_alert_at > deviation_cooldown_s
    )
    if should_alert:
        session.last_deviation_alert_at = now
        session.deviation_count += 1

    # 4. Advance step
    if best_idx > session.current_step_index:
        if session.last_announced_step < session.current_step_index:
            session.last_announced_step = session.current_step_index
        session.current_step_index = best_idx

    # 5. Progress + completion
    if not route.steps:
        return {
            "current_step_index": 0,
            "current_step_instruction": "",
            "current_step_voice_text": "",
            "current_step_distance_m": 0.0,
            "current_step_name": "",
            "is_deviated": False,
            "deviation_m": round(deviation_m, 1),
            "should_alert_deviation": False,
            "progress_pct": 0.0,
            "is_completed": True,
            "deviation_count": session.deviation_count,
            "remaining_distance_m": 0.0,
            "remaining_duration_s": 0.0,
        }

    idx = min(session.current_step_index, len(route.steps) - 1)
    current_step = route.steps[idx]
    progress_pct = (idx / max(len(route.steps) - 1, 1)) * 100.0
    is_completed = session.current_step_index >= len(route.steps) - 1
    if is_completed and session.outcome == "in_progress":
        session.outcome = "completed"

    return {
        "current_step_index": session.current_step_index,
        "current_step_instruction": current_step.instruction_text,
        "current_step_voice_text": current_step.voice_text,
        "current_step_distance_m": current_step.distance_m,
        "current_step_name": current_step.name,
        "is_deviated": is_deviated,
        "deviation_m": round(deviation_m, 1),
        "should_alert_deviation": should_alert,
        "progress_pct": round(progress_pct, 1),
        "is_completed": is_completed,
        "deviation_count": session.deviation_count,
        "remaining_distance_m": round(
            sum(s.distance_m for s in route.steps[session.current_step_index:]), 1
        ),
        "remaining_duration_s": round(
            sum(s.duration_s for s in route.steps[session.current_step_index:]), 1
        ),
    }
