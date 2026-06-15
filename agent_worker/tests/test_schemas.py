from gemini_bridge.schemas import build_gemini_tools, tool_names


def test_gemini_tool_declarations_keep_current_tool_parity():
    expected = {
        "make_voice_call",
        "make_video_call",
        "send_chat_message",
        "read_unread_messages",
        "call_family_contact",
        "call_support_contact",
        "list_family_contacts",
        "activate_camera",
        "deactivate_camera",
        "set_medication_reminder",
        "list_medication_reminders",
        "mark_medication_taken",
        "mark_medication_skipped",
        "log_health_symptom",
        "log_health_metric",
        "set_appointment_reminder",
        "emergency_alert",
        "escalate_to_family",
        "navigate_to",
        "start_walking_navigation",
        "announce_navigation_step",
        "stop_navigation",
        "escalate_to_volunteer",
        "rate_volunteer",
        "get_location",
        "get_current_location",
        "update_location",
        "web_search",
        "answer_incoming_call",
        "decline_incoming_call",
    }

    assert set(tool_names()) == expected
    tools = build_gemini_tools()
    assert tools[0].google_search is not None
    declarations = tools[1].function_declarations
    assert {declaration.name for declaration in declarations} == expected
    set_medication = next(d for d in declarations if d.name == "set_medication_reminder")
    assert set_medication.parameters.required == ["medicine_name", "dosage", "times"]
    activate_camera = next(d for d in declarations if d.name == "activate_camera")
    assert activate_camera.parameters.required == ["reason", "consent_phrase", "consent_source"]
