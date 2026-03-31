# Family Dashboard & Volunteer — Flutter Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the Family/Caregiver experience in the Flutter app — join family, view dependent's health dashboard, manage medications, view audit log + location, and register as volunteer.

**Architecture:** Refactor `FamilyHomeScreen` from a static profile page into a 3-tab layout (Gia dinh, Tin nhan, Ca nhan). Tapping a family member navigates to a `FamilyDashboardScreen` with health/medication/audit/location tabs. All dashboard API calls go through a new `AiseeDashboardService`. Volunteer registration lives in the personal tab. Screens read tokens from `AiseeSessionStore` (matching existing pattern).

**Tech Stack:** Flutter (Dart), `http` package, `StatefulWidget` + `setState`, existing `StyleTokens` theme, `url_launcher` (for maps).

**Working directory:** `/home/eddiesngu/Desktop/Dang/aiseeapp`

**Scope note:** Volunteer `match` and `match/{match_id}/status` endpoints are excluded — those are user-initiated (the visually impaired user requests a volunteer), not family-initiated. Family members only register + toggle availability.

---

## Existing API Signatures (reference)

```
AppToast.show(String message, {bool isError})          // NO context param
AppToast.snack(BuildContext context, String message)   // context-aware variant
AiseeFamilyService.fetchFamily(String accessToken)     // NOT getFamilyMembers
AiseeFamilyService.joinFamily(String accessToken, String inviteCode)
AiseeChatService.fetchUnreadCount(String accessToken)  // NOT unreadCount
AiseeChatSocket()                                      // parameterless constructor
AiseeChatSocket.connect(String accessToken)             // token passed to connect()
ChatDetailScreen({required contact, required currentUserId, chatSocket?})
```

---

## File Structure

### New Files
| File | Responsibility |
|------|---------------|
| `lib/models/aisee_dashboard_models.dart` | Data models for dashboard API responses |
| `lib/services/aisee_dashboard_service.dart` | HTTP client for `/dashboard/{userId}/*` endpoints |
| `lib/services/aisee_volunteer_service.dart` | HTTP client for `/volunteers/register` + `/volunteers/availability` |
| `lib/screens/family_dashboard_screen.dart` | Full dashboard for one family member (4 tabs) |
| `lib/screens/family_contacts_tab.dart` | Tab 1: join family or list linked members |
| `lib/screens/family_messages_tab.dart` | Tab 2: chat list with family members |
| `lib/screens/family_personal_tab.dart` | Tab 3: profile + volunteer registration |

### Modified Files
| File | Change |
|------|--------|
| `lib/screens/family_home_screen.dart` | Rewrite: 3-tab StatefulWidget, reads token from AiseeSessionStore |
| `lib/widgets/auth_shell.dart` | No changes needed — existing constructor call stays the same |

---

## Task 1: Dashboard Models

**Files:**
- Create: `lib/models/aisee_dashboard_models.dart`

- [ ] **Step 1: Create model file with all dashboard types**

```dart
// lib/models/aisee_dashboard_models.dart

class DashboardMedication {
  const DashboardMedication({
    required this.id,
    required this.name,
    required this.dosage,
    required this.times,
    required this.frequency,
    this.active = true,
  });

  final String id;
  final String name;
  final String dosage;
  final List<String> times;
  final String frequency;
  final bool active;

  factory DashboardMedication.fromJson(Map<String, dynamic> j) {
    final List<dynamic> rawTimes = j['times'] as List<dynamic>? ?? [];
    return DashboardMedication(
      id: j['id'] as String,
      name: j['name'] as String? ?? '',
      dosage: j['dosage'] as String? ?? '',
      times: rawTimes.map((e) => e.toString()).toList(),
      frequency: j['frequency'] as String? ?? 'daily',
      active: j['active'] as bool? ?? true,
    );
  }
}

class MedWithAdherence {
  const MedWithAdherence({
    required this.medication,
    required this.takenToday,
  });

  final DashboardMedication medication;
  final bool takenToday;

  factory MedWithAdherence.fromJson(Map<String, dynamic> j) {
    return MedWithAdherence(
      medication: DashboardMedication.fromJson(
        j['medication'] as Map<String, dynamic>,
      ),
      takenToday: j['taken_today'] as bool? ?? false,
    );
  }
}

class MedStatus {
  const MedStatus({
    required this.medication,
    required this.taken,
    required this.skipped,
  });

  final DashboardMedication medication;
  final bool taken;
  final bool skipped;

  factory MedStatus.fromJson(Map<String, dynamic> j) {
    return MedStatus(
      medication: DashboardMedication.fromJson(
        j['medication'] as Map<String, dynamic>,
      ),
      taken: j['taken'] as bool? ?? false,
      skipped: j['skipped'] as bool? ?? false,
    );
  }
}

class HealthSymptom {
  const HealthSymptom({
    required this.id,
    required this.symptom,
    this.severity,
    this.notes,
    required this.loggedAt,
  });

  final String id;
  final String symptom;
  final String? severity;
  final String? notes;
  final DateTime loggedAt;

  factory HealthSymptom.fromJson(Map<String, dynamic> j) {
    return HealthSymptom(
      id: j['id'] as String,
      symptom: j['symptom'] as String? ?? '',
      severity: j['severity'] as String?,
      notes: j['notes'] as String?,
      loggedAt: DateTime.parse(j['logged_at'] as String),
    );
  }
}

class HealthMetric {
  const HealthMetric({
    required this.id,
    required this.metricType,
    required this.value,
    this.unit,
    required this.loggedAt,
  });

  final String id;
  final String metricType;
  final String value;
  final String? unit;
  final DateTime loggedAt;

  factory HealthMetric.fromJson(Map<String, dynamic> j) {
    return HealthMetric(
      id: j['id'] as String,
      metricType: j['metric_type'] as String? ?? '',
      value: j['value'] as String? ?? '',
      unit: j['unit'] as String?,
      loggedAt: DateTime.parse(j['logged_at'] as String),
    );
  }
}

class DashboardHealth {
  const DashboardHealth({
    required this.medications,
    required this.recentSymptoms,
    required this.recentMetrics,
  });

  final List<MedWithAdherence> medications;
  final List<HealthSymptom> recentSymptoms;
  final List<HealthMetric> recentMetrics;

  factory DashboardHealth.fromJson(Map<String, dynamic> j) {
    return DashboardHealth(
      medications: (j['medications'] as List<dynamic>? ?? [])
          .map((e) => MedWithAdherence.fromJson(e as Map<String, dynamic>))
          .toList(),
      recentSymptoms: (j['recent_symptoms'] as List<dynamic>? ?? [])
          .map((e) => HealthSymptom.fromJson(e as Map<String, dynamic>))
          .toList(),
      recentMetrics: (j['recent_metrics'] as List<dynamic>? ?? [])
          .map((e) => HealthMetric.fromJson(e as Map<String, dynamic>))
          .toList(),
    );
  }
}

class AuditLogEntry {
  const AuditLogEntry({
    required this.id,
    required this.toolName,
    required this.params,
    this.result,
    required this.createdAt,
  });

  final String id;
  final String toolName;
  final Map<String, dynamic> params;
  final String? result;
  final DateTime createdAt;

  factory AuditLogEntry.fromJson(Map<String, dynamic> j) {
    return AuditLogEntry(
      id: j['id'] as String,
      toolName: j['tool_name'] as String? ?? '',
      params: j['params'] as Map<String, dynamic>? ?? {},
      result: j['result'] as String?,
      createdAt: DateTime.parse(j['created_at'] as String),
    );
  }

  String get toolLabel {
    const labels = {
      'make_voice_call': 'Goi dien thoai',
      'make_video_call': 'Goi video',
      'send_chat_message': 'Gui tin nhan',
      'emergency_alert': 'SOS khan cap',
      'escalate_to_family': 'Goi nguoi than',
      'escalate_to_volunteer': 'Goi tinh nguyen vien',
      'set_medication_reminder': 'Dat nhac thuoc',
      'mark_medication_taken': 'Da uong thuoc',
      'mark_medication_skipped': 'Bo qua thuoc',
      'log_health_symptom': 'Ghi trieu chung',
      'log_health_metric': 'Ghi chi so',
      'set_appointment_reminder': 'Dat lich hen',
      'play_music': 'Phat nhac',
      'navigate_to': 'Chi duong',
      'family_create_medication': 'Nguoi than them thuoc',
      'family_update_medication': 'Nguoi than sua thuoc',
      'family_delete_medication': 'Nguoi than xoa thuoc',
    };
    return labels[toolName] ?? toolName;
  }
}

class UserLocation {
  const UserLocation({this.lat, this.lng, this.updatedAt});

  final double? lat;
  final double? lng;
  final DateTime? updatedAt;

  bool get hasLocation => lat != null && lng != null;

  factory UserLocation.fromJson(Map<String, dynamic> j) {
    return UserLocation(
      lat: (j['lat'] as num?)?.toDouble(),
      lng: (j['lng'] as num?)?.toDouble(),
      updatedAt: j['updated_at'] != null
          ? DateTime.tryParse(j['updated_at'] as String)
          : null,
    );
  }
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /home/eddiesngu/Desktop/Dang/aiseeapp && flutter analyze lib/models/aisee_dashboard_models.dart`

- [ ] **Step 3: Commit**

```bash
cd /home/eddiesngu/Desktop/Dang/aiseeapp
git add lib/models/aisee_dashboard_models.dart
git commit -m "feat: add dashboard data models"
```

---

## Task 2: Dashboard Service

**Files:**
- Create: `lib/services/aisee_dashboard_service.dart`

**Reference:** Match `_error()` pattern from `lib/services/aisee_family_service.dart:114-124` exactly.

- [ ] **Step 1: Create dashboard service**

```dart
// lib/services/aisee_dashboard_service.dart
import 'dart:convert';
import 'package:http/http.dart' as http;
import '../config/app_env.dart';
import '../models/aisee_api_models.dart';
import '../models/aisee_dashboard_models.dart';

class AiseeDashboardService {
  AiseeDashboardService({http.Client? httpClient})
      : _http = httpClient ?? http.Client();

  final http.Client _http;
  void close() => _http.close();

  Uri _uri(String path) => Uri.parse('${AppEnv.apiBaseUrl}$path');

  Map<String, String> _authHeaders(String accessToken) => {
        'Authorization': 'Bearer $accessToken',
        'Content-Type': 'application/json; charset=utf-8',
        'Accept': 'application/json',
      };

  AiseeApiException _error(http.Response res, String fallback) {
    try {
      final Map<String, dynamic> map =
          jsonDecode(utf8.decode(res.bodyBytes)) as Map<String, dynamic>;
      final String? msg = map['message'] as String?;
      final String? code = map['error'] as String?;
      return AiseeApiException(msg ?? fallback, code: code);
    } catch (_) {
      return AiseeApiException(fallback);
    }
  }

  Future<DashboardHealth> getHealth(
    String accessToken,
    String userId,
  ) async {
    final res = await _http.get(
      _uri('/api/v1/dashboard/$userId/health'),
      headers: _authHeaders(accessToken),
    );
    if (res.statusCode != 200) {
      throw _error(res, 'Khong tai duoc du lieu suc khoe');
    }
    final body = jsonDecode(utf8.decode(res.bodyBytes))
        as Map<String, dynamic>;
    return DashboardHealth.fromJson(body);
  }

  Future<List<MedStatus>> getMedications(
    String accessToken,
    String userId,
  ) async {
    final res = await _http.get(
      _uri('/api/v1/dashboard/$userId/medications'),
      headers: _authHeaders(accessToken),
    );
    if (res.statusCode != 200) {
      throw _error(res, 'Khong tai duoc danh sach thuoc');
    }
    final body = jsonDecode(utf8.decode(res.bodyBytes))
        as Map<String, dynamic>;
    final list = body['medications'] as List<dynamic>? ?? [];
    return list
        .map((e) => MedStatus.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  Future<DashboardMedication> createMedication(
    String accessToken,
    String userId, {
    required String name,
    required String dosage,
    required List<String> times,
    String frequency = 'daily',
  }) async {
    final res = await _http.post(
      _uri('/api/v1/dashboard/$userId/medications'),
      headers: _authHeaders(accessToken),
      body: jsonEncode({
        'name': name,
        'dosage': dosage,
        'times': times,
        'frequency': frequency,
      }),
    );
    if (res.statusCode != 200 && res.statusCode != 201) {
      throw _error(res, 'Khong them duoc thuoc');
    }
    final body = jsonDecode(utf8.decode(res.bodyBytes))
        as Map<String, dynamic>;
    return DashboardMedication.fromJson(body);
  }

  Future<DashboardMedication> updateMedication(
    String accessToken,
    String userId,
    String medId, {
    required String name,
    required String dosage,
    required List<String> times,
    String frequency = 'daily',
  }) async {
    final res = await _http.put(
      _uri('/api/v1/dashboard/$userId/medications/$medId'),
      headers: _authHeaders(accessToken),
      body: jsonEncode({
        'name': name,
        'dosage': dosage,
        'times': times,
        'frequency': frequency,
      }),
    );
    if (res.statusCode != 200) {
      throw _error(res, 'Khong cap nhat duoc thuoc');
    }
    final body = jsonDecode(utf8.decode(res.bodyBytes))
        as Map<String, dynamic>;
    return DashboardMedication.fromJson(body);
  }

  Future<void> deleteMedication(
    String accessToken,
    String userId,
    String medId,
  ) async {
    final res = await _http.delete(
      _uri('/api/v1/dashboard/$userId/medications/$medId'),
      headers: _authHeaders(accessToken),
    );
    if (res.statusCode != 200 && res.statusCode != 204) {
      throw _error(res, 'Khong xoa duoc thuoc');
    }
  }

  Future<List<AuditLogEntry>> getAuditLog(
    String accessToken,
    String userId, {
    int page = 1,
    int limit = 20,
  }) async {
    final res = await _http.get(
      _uri('/api/v1/dashboard/$userId/audit-log'
          '?page=$page&limit=$limit'),
      headers: _authHeaders(accessToken),
    );
    if (res.statusCode != 200) {
      throw _error(res, 'Khong tai duoc lich su hoat dong');
    }
    final list = jsonDecode(utf8.decode(res.bodyBytes))
        as List<dynamic>;
    return list
        .map((e) => AuditLogEntry.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  Future<UserLocation> getLocation(
    String accessToken,
    String userId,
  ) async {
    final res = await _http.get(
      _uri('/api/v1/dashboard/$userId/location'),
      headers: _authHeaders(accessToken),
    );
    if (res.statusCode != 200) {
      throw _error(res, 'Khong tai duoc vi tri');
    }
    final body = jsonDecode(utf8.decode(res.bodyBytes))
        as Map<String, dynamic>;
    return UserLocation.fromJson(body);
  }
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /home/eddiesngu/Desktop/Dang/aiseeapp && flutter analyze lib/services/aisee_dashboard_service.dart`

- [ ] **Step 3: Commit**

```bash
cd /home/eddiesngu/Desktop/Dang/aiseeapp
git add lib/services/aisee_dashboard_service.dart
git commit -m "feat: add dashboard service for family endpoints"
```

---

## Task 3: Volunteer Service

**Files:**
- Create: `lib/services/aisee_volunteer_service.dart`

- [ ] **Step 1: Create volunteer service**

```dart
// lib/services/aisee_volunteer_service.dart
import 'dart:convert';
import 'package:http/http.dart' as http;
import '../config/app_env.dart';
import '../models/aisee_api_models.dart';

class AiseeVolunteerService {
  AiseeVolunteerService({http.Client? httpClient})
      : _http = httpClient ?? http.Client();

  final http.Client _http;
  void close() => _http.close();

  Uri _uri(String path) => Uri.parse('${AppEnv.apiBaseUrl}$path');

  Map<String, String> _authHeaders(String accessToken) => {
        'Authorization': 'Bearer $accessToken',
        'Content-Type': 'application/json; charset=utf-8',
        'Accept': 'application/json',
      };

  AiseeApiException _error(http.Response res, String fallback) {
    try {
      final Map<String, dynamic> map =
          jsonDecode(utf8.decode(res.bodyBytes)) as Map<String, dynamic>;
      final String? msg = map['message'] as String?;
      final String? code = map['error'] as String?;
      return AiseeApiException(msg ?? fallback, code: code);
    } catch (_) {
      return AiseeApiException(fallback);
    }
  }

  Future<void> register(String accessToken) async {
    final res = await _http.post(
      _uri('/api/v1/volunteers/register'),
      headers: _authHeaders(accessToken),
    );
    if (res.statusCode != 200 && res.statusCode != 201) {
      throw _error(res, 'Khong dang ky tinh nguyen vien duoc');
    }
  }

  Future<void> setAvailability(
    String accessToken, {
    required bool available,
  }) async {
    final res = await _http.put(
      _uri('/api/v1/volunteers/availability'),
      headers: _authHeaders(accessToken),
      body: jsonEncode({'available': available}),
    );
    if (res.statusCode != 200) {
      throw _error(res, 'Khong cap nhat trang thai duoc');
    }
  }
}
```

- [ ] **Step 2: Verify & commit**

```bash
cd /home/eddiesngu/Desktop/Dang/aiseeapp
flutter analyze lib/services/aisee_volunteer_service.dart
git add lib/services/aisee_volunteer_service.dart
git commit -m "feat: add volunteer service for registration and availability"
```

---

## Task 4: Family Dashboard Screen

**Files:**
- Create: `lib/screens/family_dashboard_screen.dart`

**Note:** Created BEFORE tabs so tabs can import it without forward-dependency errors.

- [ ] **Step 1: Create dashboard screen**

```dart
// lib/screens/family_dashboard_screen.dart
import 'package:flutter/material.dart';
import 'package:url_launcher/url_launcher.dart';
import '../models/aisee_dashboard_models.dart';
import '../models/aisee_family_member.dart';
import '../services/aisee_dashboard_service.dart';
import '../theme/style_tokens.dart';
import '../utils/app_toast.dart';

class FamilyDashboardScreen extends StatefulWidget {
  const FamilyDashboardScreen({
    required this.accessToken,
    required this.member,
    super.key,
  });

  final String accessToken;
  final AiseeFamilyMember member;

  @override
  State<FamilyDashboardScreen> createState() =>
      _FamilyDashboardScreenState();
}

class _FamilyDashboardScreenState extends State<FamilyDashboardScreen> {
  final _dashService = AiseeDashboardService();
  int _currentTab = 0;

  DashboardHealth? _health;
  List<MedStatus> _meds = [];
  List<AuditLogEntry> _logs = [];
  UserLocation? _location;

  bool _loadingHealth = true;
  bool _loadingMeds = true;
  bool _loadingLogs = true;
  bool _loadingLocation = true;

  @override
  void initState() {
    super.initState();
    _loadAll();
  }

  @override
  void dispose() {
    _dashService.close();
    super.dispose();
  }

  Future<void> _loadAll() async {
    await Future.wait([
      _loadHealth(),
      _loadMeds(),
      _loadLogs(),
      _loadLocation(),
    ]);
  }

  Future<void> _loadHealth() async {
    setState(() => _loadingHealth = true);
    try {
      final health = await _dashService.getHealth(
        widget.accessToken, widget.member.id,
      );
      if (!mounted) return;
      setState(() { _health = health; _loadingHealth = false; });
    } catch (_) {
      if (mounted) setState(() => _loadingHealth = false);
    }
  }

  Future<void> _loadMeds() async {
    setState(() => _loadingMeds = true);
    try {
      final meds = await _dashService.getMedications(
        widget.accessToken, widget.member.id,
      );
      if (!mounted) return;
      setState(() { _meds = meds; _loadingMeds = false; });
    } catch (_) {
      if (mounted) setState(() => _loadingMeds = false);
    }
  }

  Future<void> _loadLogs() async {
    setState(() => _loadingLogs = true);
    try {
      final logs = await _dashService.getAuditLog(
        widget.accessToken, widget.member.id,
      );
      if (!mounted) return;
      setState(() { _logs = logs; _loadingLogs = false; });
    } catch (_) {
      if (mounted) setState(() => _loadingLogs = false);
    }
  }

  Future<void> _loadLocation() async {
    setState(() => _loadingLocation = true);
    try {
      final loc = await _dashService.getLocation(
        widget.accessToken, widget.member.id,
      );
      if (!mounted) return;
      setState(() { _location = loc; _loadingLocation = false; });
    } catch (_) {
      if (mounted) setState(() => _loadingLocation = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return DefaultTabController(
      length: 4,
      child: Scaffold(
        backgroundColor: StyleTokens.background,
        appBar: AppBar(
          title: Text(widget.member.label),
          bottom: TabBar(
            isScrollable: true,
            onTap: (i) => setState(() => _currentTab = i),
            tabs: const [
              Tab(text: 'Suc khoe'),
              Tab(text: 'Thuoc'),
              Tab(text: 'Hoat dong'),
              Tab(text: 'Vi tri'),
            ],
          ),
        ),
        body: TabBarView(
          children: [
            _buildHealthTab(),
            _buildMedsTab(),
            _buildAuditTab(),
            _buildLocationTab(),
          ],
        ),
        floatingActionButton: _currentTab == 1
            ? FloatingActionButton(
                backgroundColor: StyleTokens.buttonPrimary,
                onPressed: _showAddMedDialog,
                child: const Icon(Icons.add, color: Colors.white),
              )
            : null,
      ),
    );
  }

  // --- Health tab ---

  Widget _buildHealthTab() {
    if (_loadingHealth) {
      return const Center(child: CircularProgressIndicator());
    }
    final h = _health;
    if (h == null) return const Center(child: Text('Khong co du lieu'));
    return RefreshIndicator(
      onRefresh: _loadHealth,
      child: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          _sectionTitle('Thuoc hom nay'),
          if (h.medications.isEmpty)
            _emptyCard('Chua co thuoc nao')
          else
            ...h.medications.map(_buildMedAdherenceCard),
          const SizedBox(height: 16),
          _sectionTitle('Trieu chung gan day'),
          if (h.recentSymptoms.isEmpty)
            _emptyCard('Khong co trieu chung')
          else
            ...h.recentSymptoms.map(_buildSymptomCard),
          const SizedBox(height: 16),
          _sectionTitle('Chi so suc khoe'),
          if (h.recentMetrics.isEmpty)
            _emptyCard('Khong co chi so')
          else
            ...h.recentMetrics.map(_buildMetricCard),
        ],
      ),
    );
  }

  Widget _buildMedAdherenceCard(MedWithAdherence m) {
    return Card(
      margin: const EdgeInsets.only(bottom: 8),
      child: ListTile(
        leading: Icon(
          m.takenToday ? Icons.check_circle : Icons.circle_outlined,
          color: m.takenToday ? Colors.green : Colors.orange,
        ),
        title: Text(m.medication.name),
        subtitle: Text(
          '${m.medication.dosage} - ${m.medication.times.join(", ")}',
        ),
      ),
    );
  }

  Widget _buildSymptomCard(HealthSymptom s) {
    return Card(
      margin: const EdgeInsets.only(bottom: 8),
      child: ListTile(
        leading: const Icon(Icons.healing, color: Colors.orange),
        title: Text(s.symptom),
        subtitle: Text([
          if (s.severity != null) s.severity!,
          _formatTime(s.loggedAt),
        ].join(' - ')),
      ),
    );
  }

  Widget _buildMetricCard(HealthMetric m) {
    return Card(
      margin: const EdgeInsets.only(bottom: 8),
      child: ListTile(
        leading: const Icon(Icons.monitor_heart, color: Colors.blue),
        title: Text('${m.metricType}: ${m.value} ${m.unit ?? ""}'),
        subtitle: Text(_formatTime(m.loggedAt)),
      ),
    );
  }

  // --- Medications tab ---

  Widget _buildMedsTab() {
    if (_loadingMeds) {
      return const Center(child: CircularProgressIndicator());
    }
    if (_meds.isEmpty) {
      return RefreshIndicator(
        onRefresh: _loadMeds,
        child: ListView(
          children: const [
            SizedBox(height: 200),
            Center(child: Text('Chua co thuoc nao')),
          ],
        ),
      );
    }
    return RefreshIndicator(
      onRefresh: _loadMeds,
      child: ListView.builder(
        padding: const EdgeInsets.all(16),
        itemCount: _meds.length,
        itemBuilder: (context, index) {
          final m = _meds[index];
          return Card(
            margin: const EdgeInsets.only(bottom: 8),
            child: ListTile(
              leading: Icon(
                m.taken
                    ? Icons.check_circle
                    : m.skipped
                        ? Icons.cancel
                        : Icons.circle_outlined,
                color: m.taken
                    ? Colors.green
                    : m.skipped
                        ? Colors.red
                        : Colors.grey,
              ),
              title: Text(m.medication.name),
              subtitle: Text(
                '${m.medication.dosage} - '
                '${m.medication.times.join(", ")}',
              ),
              trailing: PopupMenuButton<String>(
                onSelected: (action) =>
                    _onMedAction(action, m.medication),
                itemBuilder: (_) => const [
                  PopupMenuItem(value: 'edit', child: Text('Sua')),
                  PopupMenuItem(value: 'delete', child: Text('Xoa')),
                ],
              ),
            ),
          );
        },
      ),
    );
  }

  Future<void> _onMedAction(
    String action, DashboardMedication med,
  ) async {
    if (action == 'delete') {
      final confirm = await showDialog<bool>(
        context: context,
        builder: (_) => AlertDialog(
          title: const Text('Xoa thuoc'),
          content: Text('Ban co chac muon xoa ${med.name}?'),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(context, false),
              child: const Text('Huy'),
            ),
            TextButton(
              onPressed: () => Navigator.pop(context, true),
              child: const Text('Xoa',
                  style: TextStyle(color: Colors.red)),
            ),
          ],
        ),
      );
      if (confirm != true) return;
      try {
        await _dashService.deleteMedication(
          widget.accessToken, widget.member.id, med.id,
        );
        await _loadMeds();
        if (mounted) AppToast.show('Da xoa ${med.name}');
      } catch (e) {
        if (mounted) AppToast.show(e.toString(), isError: true);
      }
    } else if (action == 'edit') {
      _showEditMedDialog(med);
    }
  }

  Future<void> _showAddMedDialog() async {
    final result = await _showMedForm();
    if (result == null) return;
    try {
      await _dashService.createMedication(
        widget.accessToken, widget.member.id,
        name: result['name']!,
        dosage: result['dosage']!,
        times: result['times']!.split(',').map((e) => e.trim()).toList(),
        frequency: result['frequency'] ?? 'daily',
      );
      await _loadMeds();
      if (mounted) AppToast.show('Da them thuoc');
    } catch (e) {
      if (mounted) AppToast.show(e.toString(), isError: true);
    }
  }

  Future<void> _showEditMedDialog(DashboardMedication med) async {
    final result = await _showMedForm(
      name: med.name,
      dosage: med.dosage,
      times: med.times.join(', '),
      frequency: med.frequency,
    );
    if (result == null) return;
    try {
      await _dashService.updateMedication(
        widget.accessToken, widget.member.id, med.id,
        name: result['name']!,
        dosage: result['dosage']!,
        times: result['times']!.split(',').map((e) => e.trim()).toList(),
        frequency: result['frequency'] ?? 'daily',
      );
      await _loadMeds();
      if (mounted) AppToast.show('Da cap nhat thuoc');
    } catch (e) {
      if (mounted) AppToast.show(e.toString(), isError: true);
    }
  }

  Future<Map<String, String>?> _showMedForm({
    String name = '', String dosage = '',
    String times = '', String frequency = 'daily',
  }) async {
    final nameC = TextEditingController(text: name);
    final dosageC = TextEditingController(text: dosage);
    final timesC = TextEditingController(text: times);
    final freqC = TextEditingController(text: frequency);

    final result = await showDialog<Map<String, String>>(
      context: context,
      builder: (_) => AlertDialog(
        title: Text(name.isEmpty ? 'Them thuoc' : 'Sua thuoc'),
        content: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              TextField(controller: nameC,
                  decoration: const InputDecoration(labelText: 'Ten thuoc')),
              TextField(controller: dosageC,
                  decoration: const InputDecoration(labelText: 'Lieu luong')),
              TextField(controller: timesC,
                  decoration: const InputDecoration(
                      labelText: 'Gio uong (VD: 07:00, 21:00)')),
              TextField(controller: freqC,
                  decoration: const InputDecoration(
                      labelText: 'Tan suat (daily)')),
            ],
          ),
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context),
              child: const Text('Huy')),
          TextButton(
            onPressed: () {
              if (nameC.text.trim().isEmpty) return;
              Navigator.pop(context, {
                'name': nameC.text.trim(),
                'dosage': dosageC.text.trim(),
                'times': timesC.text.trim(),
                'frequency': freqC.text.trim(),
              });
            },
            child: const Text('Luu'),
          ),
        ],
      ),
    );
    nameC.dispose(); dosageC.dispose();
    timesC.dispose(); freqC.dispose();
    return result;
  }

  // --- Audit log tab ---

  Widget _buildAuditTab() {
    if (_loadingLogs) {
      return const Center(child: CircularProgressIndicator());
    }
    if (_logs.isEmpty) {
      return const Center(child: Text('Chua co hoat dong nao'));
    }
    return RefreshIndicator(
      onRefresh: _loadLogs,
      child: ListView.builder(
        padding: const EdgeInsets.all(16),
        itemCount: _logs.length,
        itemBuilder: (context, index) {
          final log = _logs[index];
          return Card(
            margin: const EdgeInsets.only(bottom: 8),
            child: ListTile(
              leading: _auditIcon(log.toolName),
              title: Text(log.toolLabel),
              subtitle: Text(_formatTime(log.createdAt)),
              onTap: () => _showLogDetail(log),
            ),
          );
        },
      ),
    );
  }

  Widget _auditIcon(String toolName) {
    if (toolName.contains('call')) {
      return const Icon(Icons.phone, color: Colors.green);
    }
    if (toolName.contains('message')) {
      return const Icon(Icons.chat, color: Colors.blue);
    }
    if (toolName.contains('emergency') || toolName.contains('sos')) {
      return const Icon(Icons.warning, color: Colors.red);
    }
    if (toolName.contains('medication') || toolName.contains('med')) {
      return const Icon(Icons.medication, color: Colors.purple);
    }
    return const Icon(Icons.history, color: Colors.grey);
  }

  void _showLogDetail(AuditLogEntry log) {
    showDialog<void>(
      context: context,
      builder: (_) => AlertDialog(
        title: Text(log.toolLabel),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('Thoi gian: ${_formatTime(log.createdAt)}'),
            if (log.params.isNotEmpty) ...[
              const SizedBox(height: 8),
              Text('Chi tiet: ${log.params.entries.map(
                    (e) => '${e.key}: ${e.value}',
                  ).join(', ')}'),
            ],
            if (log.result != null) ...[
              const SizedBox(height: 8),
              Text('Ket qua: ${log.result}'),
            ],
          ],
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context),
              child: const Text('Dong')),
        ],
      ),
    );
  }

  // --- Location tab ---

  Widget _buildLocationTab() {
    if (_loadingLocation) {
      return const Center(child: CircularProgressIndicator());
    }
    final loc = _location;
    if (loc == null || !loc.hasLocation) {
      return const Center(child: Text('Chua co vi tri'));
    }
    return RefreshIndicator(
      onRefresh: _loadLocation,
      child: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          Card(
            child: Padding(
              padding: const EdgeInsets.all(20),
              child: Column(children: [
                const Icon(Icons.location_on, size: 48, color: Colors.red),
                const SizedBox(height: 12),
                Text(
                  '${loc.lat!.toStringAsFixed(6)}, '
                  '${loc.lng!.toStringAsFixed(6)}',
                  style: StyleTokens.titleScreen().copyWith(fontSize: 16),
                ),
                if (loc.updatedAt != null) ...[
                  const SizedBox(height: 8),
                  Text('Cap nhat: ${_formatTime(loc.updatedAt!)}',
                      style: StyleTokens.bodySmall()),
                ],
                const SizedBox(height: 16),
                ElevatedButton.icon(
                  onPressed: () => _openMaps(loc.lat!, loc.lng!),
                  icon: const Icon(Icons.map),
                  label: const Text('Mo ban do'),
                  style: ElevatedButton.styleFrom(
                    backgroundColor: StyleTokens.buttonPrimary,
                    foregroundColor: Colors.white,
                  ),
                ),
              ]),
            ),
          ),
        ],
      ),
    );
  }

  Future<void> _openMaps(double lat, double lng) async {
    final uri = Uri.parse(
      'https://www.google.com/maps/search/?api=1&query=$lat,$lng',
    );
    if (await canLaunchUrl(uri)) {
      await launchUrl(uri, mode: LaunchMode.externalApplication);
    }
  }

  // --- Helpers ---

  Widget _sectionTitle(String text) => Padding(
        padding: const EdgeInsets.only(bottom: 8),
        child: Text(text,
            style: StyleTokens.titleScreen().copyWith(fontSize: 16)),
      );

  Widget _emptyCard(String text) => Card(
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Center(child: Text(text, style: StyleTokens.bodySmall())),
        ),
      );

  String _formatTime(DateTime dt) {
    final local = dt.toLocal();
    return '${local.day}/${local.month}/${local.year} '
        '${local.hour.toString().padLeft(2, "0")}:'
        '${local.minute.toString().padLeft(2, "0")}';
  }
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /home/eddiesngu/Desktop/Dang/aiseeapp && flutter analyze lib/screens/family_dashboard_screen.dart`

- [ ] **Step 3: Commit**

```bash
cd /home/eddiesngu/Desktop/Dang/aiseeapp
git add lib/screens/family_dashboard_screen.dart
git commit -m "feat: add family dashboard screen with health, meds, audit, location"
```

---

## Task 5: Family Contacts Tab

**Files:**
- Create: `lib/screens/family_contacts_tab.dart`

**Behavior:** Empty state = "Tham gia gia dinh" button with invite code input. With members = list, tap -> FamilyDashboardScreen.

- [ ] **Step 1: Create family contacts tab**

```dart
// lib/screens/family_contacts_tab.dart
import 'package:flutter/material.dart';
import '../models/aisee_family_member.dart';
import '../services/aisee_family_service.dart';
import '../services/aisee_session_store.dart';
import '../theme/style_tokens.dart';
import '../utils/app_toast.dart';
import 'family_dashboard_screen.dart';

class FamilyContactsTab extends StatefulWidget {
  const FamilyContactsTab({super.key});

  @override
  State<FamilyContactsTab> createState() => _FamilyContactsTabState();
}

class _FamilyContactsTabState extends State<FamilyContactsTab> {
  final _familyService = AiseeFamilyService();
  List<AiseeFamilyMember> _members = [];
  bool _loading = true;
  String? _error;

  final _codeController = TextEditingController();
  bool _joining = false;

  @override
  void initState() {
    super.initState();
    _loadMembers();
  }

  @override
  void dispose() {
    _codeController.dispose();
    _familyService.close();
    super.dispose();
  }

  Future<String?> _token() async {
    final stored = await AiseeSessionStore.instance.read();
    return stored?.accessToken;
  }

  Future<void> _loadMembers() async {
    setState(() { _loading = true; _error = null; });
    try {
      final token = await _token();
      if (token == null) return;
      final members = await _familyService.fetchFamily(token);
      if (!mounted) return;
      setState(() { _members = members; _loading = false; });
    } catch (e) {
      if (!mounted) return;
      setState(() { _error = e.toString(); _loading = false; });
    }
  }

  Future<void> _joinFamily() async {
    final code = _codeController.text.trim();
    if (code.isEmpty) return;
    setState(() => _joining = true);
    try {
      final token = await _token();
      if (token == null) return;
      await _familyService.joinFamily(token, code);
      _codeController.clear();
      if (!mounted) return;
      AppToast.show('Da tham gia gia dinh');
      await _loadMembers();
    } catch (e) {
      if (!mounted) return;
      AppToast.show(e.toString(), isError: true);
    } finally {
      if (mounted) setState(() => _joining = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      return const Center(child: CircularProgressIndicator());
    }
    if (_error != null) {
      return Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(_error!, style: StyleTokens.bodySmall()),
            const SizedBox(height: 12),
            TextButton(onPressed: _loadMembers,
                child: const Text('Thu lai')),
          ],
        ),
      );
    }
    if (_members.isEmpty) return _buildJoinView();
    return _buildMemberList();
  }

  Widget _buildJoinView() {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 32),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(Icons.family_restroom, size: 64,
              color: StyleTokens.textPrimary.withAlpha(77)),
          const SizedBox(height: 16),
          Text('Chua co nguoi than nao',
              style: StyleTokens.titleScreen().copyWith(fontSize: 18)),
          const SizedBox(height: 8),
          Text('Nhap ma moi de tham gia gia dinh',
              style: StyleTokens.bodySmall()),
          const SizedBox(height: 24),
          TextField(
            controller: _codeController,
            textAlign: TextAlign.center,
            decoration: InputDecoration(
              hintText: 'Nhap ma moi',
              border: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(12)),
            ),
          ),
          const SizedBox(height: 16),
          SizedBox(
            width: double.infinity, height: 48,
            child: ElevatedButton(
              onPressed: _joining ? null : _joinFamily,
              style: ElevatedButton.styleFrom(
                backgroundColor: StyleTokens.buttonPrimary,
                shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(12)),
              ),
              child: _joining
                  ? const SizedBox(width: 20, height: 20,
                      child: CircularProgressIndicator(
                          strokeWidth: 2, color: Colors.white))
                  : const Text('Tham gia gia dinh',
                      style: TextStyle(color: Colors.white,
                          fontWeight: FontWeight.w600)),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildMemberList() {
    return RefreshIndicator(
      onRefresh: _loadMembers,
      child: ListView.separated(
        padding: const EdgeInsets.all(16),
        itemCount: _members.length,
        separatorBuilder: (_, __) => const SizedBox(height: 8),
        itemBuilder: (context, index) {
          final member = _members[index];
          return ListTile(
            leading: CircleAvatar(
              backgroundColor: StyleTokens.buttonPrimary.withAlpha(26),
              child: Text(member.label[0].toUpperCase(),
                  style: TextStyle(color: StyleTokens.buttonPrimary,
                      fontWeight: FontWeight.w700)),
            ),
            title: Text(member.label,
                style: StyleTokens.bodySmall().copyWith(
                    fontWeight: FontWeight.w600, fontSize: 15)),
            subtitle: Text(member.email,
                style: StyleTokens.bodySmall().copyWith(fontSize: 12)),
            trailing: const Icon(Icons.chevron_right),
            shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(12)),
            tileColor: Colors.white,
            onTap: () async {
              final token = await _token();
              if (token == null || !mounted) return;
              Navigator.of(context).push(MaterialPageRoute<void>(
                builder: (_) => FamilyDashboardScreen(
                  accessToken: token, member: member,
                ),
              ));
            },
          );
        },
      ),
    );
  }
}
```

- [ ] **Step 2: Verify & commit**

```bash
cd /home/eddiesngu/Desktop/Dang/aiseeapp
flutter analyze lib/screens/family_contacts_tab.dart
git add lib/screens/family_contacts_tab.dart
git commit -m "feat: add family contacts tab with join flow"
```

---

## Task 6: Family Messages Tab

**Files:**
- Create: `lib/screens/family_messages_tab.dart`

- [ ] **Step 1: Create family messages tab**

```dart
// lib/screens/family_messages_tab.dart
import 'package:flutter/material.dart';
import '../models/aisee_api_models.dart';
import '../models/aisee_family_member.dart';
import '../services/aisee_chat_service.dart';
import '../services/aisee_chat_socket.dart';
import '../services/aisee_family_service.dart';
import '../services/aisee_session_store.dart';
import '../theme/style_tokens.dart';
import 'chat_detail_screen.dart';

class FamilyMessagesTab extends StatefulWidget {
  const FamilyMessagesTab({
    required this.currentUserId,
    required this.chatSocket,
    super.key,
  });

  final String currentUserId;
  final AiseeChatSocket chatSocket;

  @override
  State<FamilyMessagesTab> createState() => _FamilyMessagesTabState();
}

class _FamilyMessagesTabState extends State<FamilyMessagesTab> {
  final _familyService = AiseeFamilyService();
  final _chatService = AiseeChatService();
  List<AiseeFamilyMember> _members = [];
  bool _loading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void dispose() {
    _familyService.close();
    _chatService.close();
    super.dispose();
  }

  Future<void> _load() async {
    setState(() { _loading = true; _error = null; });
    try {
      final stored = await AiseeSessionStore.instance.read();
      if (stored == null) return;
      final members = await _familyService.fetchFamily(
        stored.accessToken,
      );
      if (!mounted) return;
      setState(() { _members = members; _loading = false; });
    } catch (e) {
      if (!mounted) return;
      setState(() { _error = e.toString(); _loading = false; });
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      return const Center(child: CircularProgressIndicator());
    }
    if (_error != null) {
      return Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(_error!, style: StyleTokens.bodySmall()),
            const SizedBox(height: 12),
            TextButton(onPressed: _load,
                child: const Text('Thu lai')),
          ],
        ),
      );
    }
    if (_members.isEmpty) {
      return Center(
        child: Text('Chua co nguoi than de nhan tin',
            style: StyleTokens.bodySmall()),
      );
    }
    return RefreshIndicator(
      onRefresh: _load,
      child: ListView.separated(
        padding: const EdgeInsets.all(16),
        itemCount: _members.length,
        separatorBuilder: (_, __) => const SizedBox(height: 8),
        itemBuilder: (context, index) {
          final member = _members[index];
          return ListTile(
            leading: CircleAvatar(
              backgroundColor: StyleTokens.buttonPrimary.withAlpha(26),
              child: Text(member.label[0].toUpperCase(),
                  style: TextStyle(color: StyleTokens.buttonPrimary,
                      fontWeight: FontWeight.w700)),
            ),
            title: Text(member.label,
                style: StyleTokens.bodySmall().copyWith(
                    fontWeight: FontWeight.w600, fontSize: 15)),
            trailing: const Icon(Icons.chat_bubble_outline),
            shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(12)),
            tileColor: Colors.white,
            onTap: () {
              Navigator.of(context).push(MaterialPageRoute<void>(
                builder: (_) => ChatDetailScreen(
                  contact: member,
                  currentUserId: widget.currentUserId,
                  chatSocket: widget.chatSocket,
                ),
              ));
            },
          );
        },
      ),
    );
  }
}
```

- [ ] **Step 2: Verify & commit**

```bash
cd /home/eddiesngu/Desktop/Dang/aiseeapp
flutter analyze lib/screens/family_messages_tab.dart
git add lib/screens/family_messages_tab.dart
git commit -m "feat: add family messages tab"
```

---

## Task 7: Family Personal Tab

**Files:**
- Create: `lib/screens/family_personal_tab.dart`

- [ ] **Step 1: Create family personal tab**

```dart
// lib/screens/family_personal_tab.dart
import 'package:flutter/material.dart';
import '../models/aisee_api_models.dart';
import '../services/aisee_session_store.dart';
import '../services/aisee_volunteer_service.dart';
import '../theme/style_tokens.dart';
import '../utils/app_toast.dart';

class FamilyPersonalTab extends StatefulWidget {
  const FamilyPersonalTab({
    required this.profile,
    required this.onSignOut,
    super.key,
  });

  final AiseeUserProfile profile;
  final VoidCallback onSignOut;

  @override
  State<FamilyPersonalTab> createState() => _FamilyPersonalTabState();
}

class _FamilyPersonalTabState extends State<FamilyPersonalTab> {
  final _volunteerService = AiseeVolunteerService();
  late bool _isVolunteer;
  bool _available = false;
  bool _registering = false;
  bool _toggling = false;

  @override
  void initState() {
    super.initState();
    _isVolunteer = widget.profile.isVolunteer;
  }

  @override
  void dispose() {
    _volunteerService.close();
    super.dispose();
  }

  Future<String?> _token() async {
    final stored = await AiseeSessionStore.instance.read();
    return stored?.accessToken;
  }

  Future<void> _register() async {
    setState(() => _registering = true);
    try {
      final token = await _token();
      if (token == null) return;
      await _volunteerService.register(token);
      if (!mounted) return;
      setState(() { _isVolunteer = true; _available = true; });
      AppToast.show('Da dang ky tinh nguyen vien');
    } catch (e) {
      if (!mounted) return;
      AppToast.show(e.toString(), isError: true);
    } finally {
      if (mounted) setState(() => _registering = false);
    }
  }

  Future<void> _toggleAvailability(bool value) async {
    setState(() => _toggling = true);
    try {
      final token = await _token();
      if (token == null) return;
      await _volunteerService.setAvailability(
        token, available: value,
      );
      if (!mounted) return;
      setState(() => _available = value);
    } catch (e) {
      if (!mounted) return;
      AppToast.show(e.toString(), isError: true);
    } finally {
      if (mounted) setState(() => _toggling = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return ListView(
      padding: const EdgeInsets.all(20),
      children: [
        // Profile card
        Container(
          padding: const EdgeInsets.all(20),
          decoration: BoxDecoration(
            color: Colors.white,
            borderRadius: BorderRadius.circular(16),
          ),
          child: Column(children: [
            CircleAvatar(
              radius: 36,
              backgroundColor: StyleTokens.buttonPrimary.withAlpha(26),
              child: Text(
                (widget.profile.displayName ?? widget.profile.email)
                    [0].toUpperCase(),
                style: TextStyle(fontSize: 28,
                    fontWeight: FontWeight.w700,
                    color: StyleTokens.buttonPrimary),
              ),
            ),
            const SizedBox(height: 12),
            Text(widget.profile.displayName ?? '',
                style: StyleTokens.titleScreen().copyWith(fontSize: 18)),
            const SizedBox(height: 4),
            Text(widget.profile.email,
                style: StyleTokens.bodySmall()),
          ]),
        ),
        const SizedBox(height: 16),

        // Volunteer section
        Container(
          padding: const EdgeInsets.all(20),
          decoration: BoxDecoration(
            color: Colors.white,
            borderRadius: BorderRadius.circular(16),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('Tinh nguyen vien',
                  style: StyleTokens.titleScreen().copyWith(fontSize: 16)),
              const SizedBox(height: 8),
              Text('Ho tro nguoi khiem thi khi nguoi than '
                  'khong the tra loi',
                  style: StyleTokens.bodySmall()),
              const SizedBox(height: 16),
              if (!_isVolunteer)
                SizedBox(
                  width: double.infinity, height: 44,
                  child: ElevatedButton.icon(
                    onPressed: _registering ? null : _register,
                    icon: _registering
                        ? const SizedBox(width: 18, height: 18,
                            child: CircularProgressIndicator(
                                strokeWidth: 2, color: Colors.white))
                        : const Icon(Icons.volunteer_activism,
                            color: Colors.white),
                    label: const Text('Dang ky tinh nguyen vien',
                        style: TextStyle(color: Colors.white)),
                    style: ElevatedButton.styleFrom(
                      backgroundColor: StyleTokens.buttonPrimary,
                      shape: RoundedRectangleBorder(
                          borderRadius: BorderRadius.circular(12)),
                    ),
                  ),
                )
              else
                SwitchListTile(
                  title: const Text('San sang ho tro'),
                  subtitle: Text(_available
                      ? 'Ban se nhan cuoc goi ho tro'
                      : 'Tam nghi, khong nhan cuoc goi'),
                  value: _available,
                  onChanged: _toggling ? null : _toggleAvailability,
                  activeColor: StyleTokens.buttonPrimary,
                  contentPadding: EdgeInsets.zero,
                ),
            ],
          ),
        ),
        const SizedBox(height: 16),

        // Sign out
        TextButton(
          onPressed: widget.onSignOut,
          child: Text('Dang xuat',
              style: StyleTokens.bodySmall().copyWith(
                  color: Colors.red, fontWeight: FontWeight.w600)),
        ),
      ],
    );
  }
}
```

- [ ] **Step 2: Verify & commit**

```bash
cd /home/eddiesngu/Desktop/Dang/aiseeapp
flutter analyze lib/screens/family_personal_tab.dart
git add lib/screens/family_personal_tab.dart
git commit -m "feat: add family personal tab with volunteer registration"
```

---

## Task 8: Rewrite FamilyHomeScreen

**Files:**
- Modify: `lib/screens/family_home_screen.dart`

**Key decisions:**
- Constructor stays compatible with `auth_shell.dart:292-298` (no `accessToken` param)
- Reads token from `AiseeSessionStore` internally (matches UserHomeScreen pattern)
- Creates `AiseeChatSocket` and connects with token from store

- [ ] **Step 1: Read current FamilyHomeScreen to confirm constructor**

Read `lib/screens/family_home_screen.dart` and `lib/widgets/auth_shell.dart:292-298`.

Current call in auth_shell.dart:
```dart
FamilyHomeScreen(
  googleUser: googleUser,
  aiseeProfile: aiseeProfile,
  aiseeLoading: aiseeLoading,
  aiseeError: aiseeError,
  onSignOut: _signOut,
)
```

- [ ] **Step 2: Rewrite FamilyHomeScreen**

Replace entire `lib/screens/family_home_screen.dart`:

```dart
// lib/screens/family_home_screen.dart
import 'dart:async';
import 'package:flutter/material.dart';
import 'package:google_sign_in/google_sign_in.dart';
import '../models/aisee_api_models.dart';
import '../services/aisee_chat_socket.dart';
import '../services/aisee_session_store.dart';
import '../theme/style_tokens.dart';
import 'family_contacts_tab.dart';
import 'family_messages_tab.dart';
import 'family_personal_tab.dart';

class FamilyHomeScreen extends StatefulWidget {
  const FamilyHomeScreen({
    this.googleUser,
    this.aiseeProfile,
    this.aiseeLoading = false,
    this.aiseeError,
    required this.onSignOut,
    super.key,
  });

  final GoogleSignInAccount? googleUser;
  final AiseeUserProfile? aiseeProfile;
  final bool aiseeLoading;
  final String? aiseeError;
  final VoidCallback onSignOut;

  @override
  State<FamilyHomeScreen> createState() => _FamilyHomeScreenState();
}

class _FamilyHomeScreenState extends State<FamilyHomeScreen> {
  int _tabIndex = 0;
  AiseeChatSocket? _chatSocket;

  @override
  void initState() {
    super.initState();
    _initSocket();
  }

  Future<void> _initSocket() async {
    final stored = await AiseeSessionStore.instance.read();
    if (stored == null || !mounted) return;
    _chatSocket = AiseeChatSocket();
    unawaited(_chatSocket!.connect(stored.accessToken));
    if (mounted) setState(() {});
  }

  @override
  void dispose() {
    _chatSocket?.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (widget.aiseeLoading) {
      return const Scaffold(
          body: Center(child: CircularProgressIndicator()));
    }
    final profile = widget.aiseeProfile;
    if (profile == null) {
      return Scaffold(
        body: Center(
          child: Text(widget.aiseeError ?? 'Khong tai duoc thong tin',
              style: StyleTokens.bodySmall()),
        ),
      );
    }

    return Scaffold(
      backgroundColor: StyleTokens.background,
      body: SafeArea(
        child: IndexedStack(
          index: _tabIndex,
          children: [
            const FamilyContactsTab(),
            FamilyMessagesTab(
              currentUserId: profile.id,
              chatSocket: _chatSocket ?? AiseeChatSocket(),
            ),
            FamilyPersonalTab(
              profile: profile,
              onSignOut: widget.onSignOut,
            ),
          ],
        ),
      ),
      bottomNavigationBar: BottomNavigationBar(
        currentIndex: _tabIndex,
        onTap: (i) => setState(() => _tabIndex = i),
        selectedItemColor: StyleTokens.buttonPrimary,
        items: const [
          BottomNavigationBarItem(
              icon: Icon(Icons.family_restroom),
              label: 'Gia dinh'),
          BottomNavigationBarItem(
              icon: Icon(Icons.chat),
              label: 'Tin nhan'),
          BottomNavigationBarItem(
              icon: Icon(Icons.person),
              label: 'Ca nhan'),
        ],
      ),
    );
  }
}
```

- [ ] **Step 3: Verify auth_shell.dart does NOT need changes**

The constructor matches: `googleUser`, `aiseeProfile`, `aiseeLoading`, `aiseeError`, `onSignOut`. No changes to auth_shell.dart.

- [ ] **Step 4: Verify full app compiles**

Run: `cd /home/eddiesngu/Desktop/Dang/aiseeapp && flutter analyze`

- [ ] **Step 5: Commit**

```bash
cd /home/eddiesngu/Desktop/Dang/aiseeapp
git add lib/screens/family_home_screen.dart
git commit -m "feat: rewrite family home screen with 3-tab layout"
```

---

## Task 9: Integration Verification

- [ ] **Step 1: Run full flutter analyze**

Run: `cd /home/eddiesngu/Desktop/Dang/aiseeapp && flutter analyze`
Expected: 0 errors

- [ ] **Step 2: Check for any remaining import mismatches**

Grep for all new file imports to verify they resolve:
```bash
cd /home/eddiesngu/Desktop/Dang/aiseeapp
rg "family_dashboard_screen|family_contacts_tab|family_messages_tab|family_personal_tab|aisee_dashboard_service|aisee_volunteer_service|aisee_dashboard_models" lib/ --files-with-matches
```

- [ ] **Step 3: Fix any compilation errors and commit**

```bash
cd /home/eddiesngu/Desktop/Dang/aiseeapp
git add -u
git commit -m "fix: resolve integration issues across family screens"
```
