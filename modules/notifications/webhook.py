"""Vesper Notification Webhook Module
Sends campaign events to Slack, Discord, and Telegram via webhooks.
"""
import json
import os
import urllib.request
import urllib.error
from datetime import datetime


def send_notification(event_type: str, event_data: dict, config: dict) -> dict:
    """Send notification to configured webhook channels."""
    results = {"sent": [], "errors": []}

    ncfg = config.get("notifications", {})
    if not ncfg.get("enabled", False):
        return {"sent": [], "errors": [], "note": "notifications disabled"}

    allowed_events = ncfg.get("events", [])
    if event_type not in allowed_events:
        return {"sent": [], "errors": [], "note": f"event '{event_type}' not in allowed_events"}

    message = _format_message(event_type, event_data)

    slack_webhook = ncfg.get("slack_webhook", "") or os.environ.get("VESPER_SLACK_WEBHOOK", "")
    if slack_webhook:
        slack_payload = json.dumps({"text": message}).encode()
        try:
            req = urllib.request.Request(slack_webhook, data=slack_payload,
                                         headers={"Content-Type": "application/json"})
            urllib.request.urlopen(req, timeout=5)
            results["sent"].append("slack")
        except urllib.error.URLError as e:
            results["errors"].append({"channel": "slack", "error": str(e)})

    discord_webhook = ncfg.get("discord_webhook", "") or os.environ.get("VESPER_DISCORD_WEBHOOK", "")
    if discord_webhook:
        discord_payload = json.dumps({
            "content": "",
            "embeds": [{
                "title": f"Vesper - {event_type}",
                "description": message,
                "color": 0xFF0000,
                "timestamp": datetime.utcnow().isoformat(),
                "footer": {"text": "Vesper Framework"},
            }],
        }).encode()
        try:
            req = urllib.request.Request(discord_webhook, data=discord_payload,
                                         headers={"Content-Type": "application/json"})
            urllib.request.urlopen(req, timeout=5)
            results["sent"].append("discord")
        except urllib.error.URLError as e:
            results["errors"].append({"channel": "discord", "error": str(e)})

    telegram_token = ncfg.get("telegram_bot_token", "") or os.environ.get("VESPER_TELEGRAM_TOKEN", "")
    telegram_chat_id = ncfg.get("telegram_chat_id", "") or os.environ.get("VESPER_TELEGRAM_CHAT_ID", "")
    if telegram_token and telegram_chat_id:
        telegram_url = f"https://api.telegram.org/bot{telegram_token}/sendMessage"
        telegram_payload = urllib.parse.urlencode({
            "chat_id": telegram_chat_id,
            "text": message,
            "parse_mode": "HTML",
        }).encode()
        try:
            req = urllib.request.Request(telegram_url, data=telegram_payload)
            urllib.request.urlopen(req, timeout=5)
            results["sent"].append("telegram")
        except urllib.error.URLError as e:
            results["errors"].append({"channel": "telegram", "error": str(e)})

    return results


def _format_message(event_type: str, data: dict) -> str:
    """Format event data into a readable notification message."""
    emoji_map = {
        "campaign_started": "🚀",
        "campaign_completed": "🏁",
        "agent_detected": "🕵️",
        "blue_team_alert": "🚨",
    }
    emoji = emoji_map.get(event_type, "📢")

    lines = [f"{emoji} Vesper Event: {event_type}"]
    lines.append(f"Timestamp: {datetime.now().isoformat()}")
    lines.append(f"Host: {os.uname().nodename}")

    for key, value in data.items():
        if isinstance(value, (str, int, float, bool)):
            lines.append(f"{key}: {value}")

    # Truncate to avoid hitting API limits
    message = "\n".join(lines)
    if len(message) > 1500:
        message = message[:1497] + "..."

    return message


import urllib.parse
