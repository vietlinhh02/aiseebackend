"""Memory manager for AI session persistence and cross-session continuity.

Handles:
- Rolling conversation summary generation (via Gemini lightweight call)
- Cross-session user fact extraction
- Periodic persistence to Go backend (PostgreSQL)
- Context restoration on reconnect
"""

from __future__ import annotations

import asyncio
import logging
import time
from dataclasses import dataclass, field
from typing import Any

from config import (
    GEMINI_API_KEY,
    MEMORY_MAX_TURNS,
    MEMORY_SAVE_INTERVAL_SECONDS,
    MEMORY_SUMMARY_INTERVAL,
)

logger = logging.getLogger("aisee-agent")

SUMMARY_PROMPT = """Tóm tắt cuộc trò chuyện sau thành 2-3 câu ngắn gọn bằng tiếng Việt.
Chỉ nêu ý chính: người dùng muốn gì, đã làm gì, có gì cần tiếp tục.
Không lặp lại nguyên văn, không thêm suy đoán.

Cuộc trò chuyện:
{conversation}

Tóm tắt:"""

FACTS_PROMPT = """Từ cuộc trò chuyện sau, trích xuất thông tin cá nhân của người dùng (tên, sở thích, thói quen, tình trạng sức khỏe, người thân, v.v.).
Chỉ trích xuất thông tin được người dùng TỰ NÓI, không suy đoán.
Trả về danh sách, mỗi dòng một fact. Nếu không có thông tin mới, trả về "NONE".

Cuộc trò chuyện:
{conversation}

Thông tin cá nhân:"""


# ── Global cap on concurrent summary generation ──
# Each session can trigger summary every 10 turns. With 100 concurrent users
# that could be 100 parallel Gemini API calls, easily overwhelming the
# outbound rate limit and stacking CPU. One semaphore per process is enough.
_SUMMARY_SEMAPHORE: asyncio.Semaphore | None = None
_SUMMARY_SEMAPHORE_LIMIT = 3


def _get_summary_semaphore() -> asyncio.Semaphore:
    global _SUMMARY_SEMAPHORE
    if _SUMMARY_SEMAPHORE is None:
        _SUMMARY_SEMAPHORE = asyncio.Semaphore(_SUMMARY_SEMAPHORE_LIMIT)
    return _SUMMARY_SEMAPHORE


@dataclass
class PersistedMemory:
    """Memory loaded from DB for cross-session continuity."""
    summary: str = ""
    recent_turns: list[dict[str, str]] = field(default_factory=list)
    user_facts: list[str] = field(default_factory=list)
    last_intent: str = ""
    pending_tasks: list[str] = field(default_factory=list)
    last_mode: str = "normal"
    found: bool = False


class MemoryManager:
    """Orchestrates memory operations for one AI session.

    Lifecycle:
    1. Created at session start
    2. load_from_backend() — restore cross-session memory
    3. add_turn() — called each user/assistant turn
    4. Periodic save (background task)
    5. save_to_backend() — called on disconnect
    """

    def __init__(
        self,
        backend: Any,
        user_id: str,
        session_id: str = "",
        *,
        summary_interval: int = MEMORY_SUMMARY_INTERVAL,
        max_turns: int = MEMORY_MAX_TURNS,
        save_interval: float = MEMORY_SAVE_INTERVAL_SECONDS,
    ) -> None:
        self.backend = backend
        self.user_id = user_id
        self.session_id = session_id or "default"
        self._summary_interval = summary_interval
        self._max_turns = max_turns
        self._save_interval = save_interval

        # In-memory state
        self.rolling_summary: str = ""
        self.recent_turns: list[dict[str, str]] = []
        self.user_facts: list[str] = []
        self.last_intent: str = ""
        self.pending_tasks: list[str] = []

        # Counters
        self._turns_since_summary: int = 0
        self._turns_since_save: int = 0
        self._last_save_at: float = time.monotonic()

        # Background task
        self._summary_task: asyncio.Task | None = None
        self._save_task: asyncio.Task | None = None
        self._periodic_save_task: asyncio.Task | None = None

    # ── Lifecycle ──

    async def load_from_backend(self) -> PersistedMemory:
        """Load previous session memory from Go backend."""
        if not self.user_id:
            return PersistedMemory()

        try:
            result = await self.backend.load_session_memory(
                user_id=self.user_id,
                session_id="",  # Load most recent, not specific session
            )
        except Exception as exc:
            logger.warning("load session memory failed: %s", exc)
            return PersistedMemory()

        if not result.get("found"):
            return PersistedMemory()

        memory = PersistedMemory(
            summary=result.get("summary", ""),
            recent_turns=result.get("turns", []),
            user_facts=result.get("facts", []),
            last_intent=result.get("last_intent", ""),
            pending_tasks=result.get("pending_tasks", []),
            last_mode=result.get("last_mode", "normal"),
            found=True,
        )

        # Restore into current state
        self.rolling_summary = memory.summary
        self.user_facts = memory.user_facts
        self.last_intent = memory.last_intent
        self.pending_tasks = memory.pending_tasks

        logger.info(
            "loaded session memory: summary=%d chars, facts=%d, turns=%d",
            len(memory.summary),
            len(memory.user_facts),
            len(memory.recent_turns),
        )
        return memory

    def start_periodic_save(self) -> None:
        """Start background periodic save task."""
        if self._periodic_save_task is not None:
            return
        self._periodic_save_task = asyncio.create_task(self._periodic_save_loop())

    async def save_to_backend(self) -> None:
        """Persist current memory state to Go backend."""
        if not self.user_id:
            return

        try:
            await self.backend.save_session_memory(
                user_id=self.user_id,
                session_id=self.session_id,
                summary=self.rolling_summary,
                recent_turns=self.recent_turns[-self._max_turns:],
                user_facts=self.user_facts,
                last_intent=self.last_intent,
                pending_tasks=self.pending_tasks,
            )
            self._last_save_at = time.monotonic()
            logger.info(
                "saved session memory: summary=%d chars, facts=%d, turns=%d",
                len(self.rolling_summary),
                len(self.user_facts),
                len(self.recent_turns),
            )
        except Exception as exc:
            logger.warning("save session memory failed: %s", exc)

    async def shutdown(self) -> None:
        """Cancel background tasks and do final save."""
        if self._periodic_save_task and not self._periodic_save_task.done():
            self._periodic_save_task.cancel()
            try:
                await self._periodic_save_task
            except asyncio.CancelledError:
                pass

        if self._summary_task and not self._summary_task.done():
            self._summary_task.cancel()
            try:
                await self._summary_task
            except asyncio.CancelledError:
                pass

        # Final save
        await self.save_to_backend()

    # ── Turn tracking ──

    def add_user_turn(self, text: str) -> None:
        """Record a user turn."""
        self.recent_turns.append({"role": "user", "text": text})
        self._trim_turns()
        self._turns_since_summary += 1
        self._turns_since_save += 1
        self._maybe_trigger_summary()

    def add_assistant_turn(self, text: str) -> None:
        """Record an assistant turn."""
        self.recent_turns.append({"role": "assistant", "text": text})
        self._trim_turns()
        self._turns_since_summary += 1
        self._turns_since_save += 1
        self._maybe_trigger_summary()

    def set_intent(self, intent: str) -> None:
        """Update last detected user intent."""
        self.last_intent = intent

    def add_pending_task(self, task: str) -> None:
        """Add an unresolved task."""
        if task not in self.pending_tasks:
            self.pending_tasks.append(task)

    def resolve_task(self, task: str) -> None:
        """Mark a task as resolved."""
        if task in self.pending_tasks:
            self.pending_tasks.remove(task)

    # ── Context for prompt injection ──

    def build_context_for_gemini(self) -> str:
        """Build context string to inject into Gemini session.

        Called on reconnect or when restoring session.
        """
        parts: list[str] = []

        if self.rolling_summary:
            parts.append(f"Tóm tắt cuộc trò chuyện trước: {self.rolling_summary}")

        if self.user_facts:
            parts.append("Thông tin đã biết về người dùng:\n" + "\n".join(
                f"- {fact}" for fact in self.user_facts[:10]
            ))

        if self.last_intent:
            parts.append(f"Ý định gần nhất: {self.last_intent}")

        if self.pending_tasks:
            parts.append(f"Chưa hoàn thành: {', '.join(self.pending_tasks[:5])}")

        if self.recent_turns:
            turns_text = []
            for t in self.recent_turns[-5:]:
                role = "User" if t["role"] == "user" else "Assistant"
                text = t["text"][:100] + ("..." if len(t["text"]) > 100 else "")
                turns_text.append(f"{role}: {text}")
            parts.append("Cuộc trò chuyện gần đây:\n" + "\n".join(turns_text))

        if not parts:
            return ""

        return (
            "=== THÔNG TIN PHIEN TRƯỚC (từ bộ nhớ) ===\n"
            + "\n\n".join(parts)
            + "\n=== HẾT THÔNG TIN ==="
        )

    def get_snapshot(self) -> dict[str, Any]:
        """Get snapshot for client-side persistence (fallback)."""
        return {
            "summary": self.rolling_summary,
            "recent_turns": self.recent_turns[-self._max_turns:],
            "user_facts": self.user_facts,
            "last_intent": self.last_intent,
            "pending_tasks": self.pending_tasks,
        }

    # ── Internal ──

    def _trim_turns(self) -> None:
        """Keep only the last max_turns."""
        if len(self.recent_turns) > self._max_turns:
            self.recent_turns = self.recent_turns[-self._max_turns:]

    def _maybe_trigger_summary(self) -> None:
        """Trigger background summary generation if interval reached."""
        if self._turns_since_summary < self._summary_interval:
            return
        if self._summary_task and not self._summary_task.done():
            return  # Already generating

        self._summary_task = asyncio.create_task(self._generate_summary())

    async def _generate_summary(self) -> None:
        """Generate rolling summary using Gemini lightweight call."""
        if not GEMINI_API_KEY:
            logger.debug("no Gemini API key, skipping summary generation")
            return

        # Cap concurrent summary calls process-wide to protect Gemini quota
        # and CPU. If the cap is saturated, this session skips its summary
        # (the next turn will try again).
        sem = _get_summary_semaphore()
        if sem.locked():
            logger.debug("summary skipped: global cap reached")
            return
        async with sem:
            try:
                from google import genai
                from google.genai import types

                client = genai.Client(api_key=GEMINI_API_KEY)

                # Build conversation text from recent turns
                conv_lines = []
                for turn in self.recent_turns[-self._summary_interval * 2:]:
                    role = "User" if turn["role"] == "user" else "Assistant"
                    conv_lines.append(f"{role}: {turn['text']}")
                conversation = "\n".join(conv_lines)

                if not conversation.strip():
                    return

                prompt = SUMMARY_PROMPT.format(conversation=conversation)

                # Use flash for speed and low cost
                response = await client.aio.models.generate_content(
                    model="gemini-2.0-flash",
                    contents=prompt,
                    config=types.GenerateContentConfig(
                        max_output_tokens=200,
                        temperature=0.3,
                    ),
                )

                new_summary = (response.text or "").strip()
                if new_summary:
                    # Merge with existing summary
                    if self.rolling_summary:
                        self.rolling_summary = f"{self.rolling_summary}\n{new_summary}"
                    else:
                        self.rolling_summary = new_summary
                    # Cap summary length
                    if len(self.rolling_summary) > 2000:
                        self.rolling_summary = self.rolling_summary[-2000:]
                    logger.info("generated summary: %d chars", len(new_summary))

                self._turns_since_summary = 0

                # Also extract facts occasionally (every other summary)
                if len(self.user_facts) < 20:
                    await self._extract_facts(client, conversation)

            except Exception as exc:
                logger.warning("summary generation failed: %s", exc)
            finally:
                self._turns_since_summary = 0

    async def _extract_facts(self, client: Any, conversation: str) -> None:
        """Extract user facts from conversation."""
        try:
            from google.genai import types

            prompt = FACTS_PROMPT.format(conversation=conversation)
            response = await client.aio.models.generate_content(
                model="gemini-2.0-flash",
                contents=prompt,
                config=types.GenerateContentConfig(
                    max_output_tokens=300,
                    temperature=0.2,
                ),
            )

            raw = (response.text or "").strip()
            if not raw or raw.upper() == "NONE":
                return

            new_facts = [
                line.strip().lstrip("- •*")
                for line in raw.split("\n")
                if line.strip() and line.strip().upper() != "NONE"
            ]

            # Deduplicate
            existing_lower = {f.lower() for f in self.user_facts}
            for fact in new_facts:
                if fact and fact.lower() not in existing_lower:
                    self.user_facts.append(fact)
                    existing_lower.add(fact.lower())

            # Cap at 30 facts
            if len(self.user_facts) > 30:
                self.user_facts = self.user_facts[-30:]

            logger.info("extracted %d new user facts", len(new_facts))

        except Exception as exc:
            logger.debug("fact extraction failed: %s", exc)

    async def _periodic_save_loop(self) -> None:
        """Background loop: save memory to backend periodically."""
        try:
            while True:
                await asyncio.sleep(self._save_interval)
                elapsed = time.monotonic() - self._last_save_at
                if elapsed >= self._save_interval and self._turns_since_save > 0:
                    await self.save_to_backend()
                    self._turns_since_save = 0
        except asyncio.CancelledError:
            return
