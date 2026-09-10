#!/usr/bin/env python3
"""MITRE ATT&CK Navigator Layer Export for Vesper Campaigns.

Generates an ATT&CK Navigator v4.9 compatible layer JSON from campaign
decisions, exploits, and recon data. Each technique used in the campaign
is scored by confidence and color-coded by tactic.

Output can be loaded directly into:
  https://mitre-attack.github.io/attack-navigator/
"""

import json
import os
from datetime import datetime, timezone
from typing import Any, Dict, List, Optional

# Tactic → color mapping (ATT&CK Navigator convention)
TACTIC_COLORS = {
    "reconnaissance": "#f5e6d3",
    "resource_development": "#e8d5b7",
    "initial_access": "#ff6666",
    "execution": "#ff9999",
    "persistence": "#66b366",
    "privilege_escalation": "#99cc99",
    "defense_evasion": "#ffcc00",
    "credential_access": "#66b3ff",
    "discovery": "#99ccff",
    "lateral_movement": "#cc99ff",
    "collection": "#ff99cc",
    "command_and_control": "#d9b3ff",
    "exfiltration": "#ffb3cc",
    "impact": "#cc0000",
}

# Known techniques with their names and tactics
TECHNIQUE_DB = {
    "T1003": ("OS Credential Dumping", "credential_access"),
    "T1005": ("Data from Local System", "collection"),
    "T1021": ("Remote Services", "lateral_movement"),
    "T1027": ("Obfuscated Files or Information", "defense_evasion"),
    "T1036": ("Masquerading", "defense_evasion"),
    "T1041": ("Exfiltration Over C2 Channel", "exfiltration"),
    "T1046": ("Network Service Discovery", "discovery"),
    "T1053": ("Scheduled Task/Job", "execution"),
    "T1055": ("Process Injection", "defense_evasion"),
    "T1059": ("Command and Scripting Interpreter", "execution"),
    "T1068": ("Exploitation for Privilege Escalation", "privilege_escalation"),
    "T1070": ("Indicator Removal", "defense_evasion"),
    "T1071": ("Application Layer Protocol", "command_and_control"),
    "T1082": ("System Information Discovery", "discovery"),
    "T1083": ("File and Directory Discovery", "discovery"),
    "T1090": ("Proxy", "command_and_control"),
    "T1110": ("Brute Force", "credential_access"),
    "T1136": ("Create Account", "persistence"),
    "T1190": ("Exploit Public-Facing Application", "initial_access"),
    "T1203": ("Exploitation for Client Execution", "execution"),
    "T1210": ("Exploitation of Remote Services", "lateral_movement"),
    "T1218": ("Signed Binary Proxy Execution", "defense_evasion"),
    "T1486": ("Data Encrypted for Impact", "impact"),
    "T1490": ("Inhibit System Recovery", "impact"),
    "T1543": ("Create or Modify System Process", "persistence"),
    "T1547": ("Boot or Logon Autostart Execution", "persistence"),
    "T1548": ("Abuse Elevation Control Mechanism", "privilege_escalation"),
    "T1562": ("Impair Defenses", "defense_evasion"),
    "T1565": ("Data Manipulation", "impact"),
    "T1566": ("Phishing", "initial_access"),
    "T1570": ("Lateral Tool Transfer", "lateral_movement"),
    "T1574": ("Hijack Execution Flow", "defense_evasion"),
    "T1593": ("Search Open Websites/Domains", "reconnaissance"),
    "T1595": ("Active Scanning", "reconnaissance"),
    "T1611": ("Escape to Host", "defense_evasion"),
}


def generate_navigator_layer(
    campaign_name: str = "Vesper Campaign",
    campaign_id: str = "",
    techniques: Optional[List[Dict[str, Any]]] = None,
    description: str = "",
) -> Dict[str, Any]:
    """Generate an ATT&CK Navigator v4.9 layer JSON.

    Args:
        campaign_name: Name of the campaign
        campaign_id: Campaign identifier
        techniques: List of technique dicts with keys:
            - technique_id: str (e.g. "T1210")
            - tactic: str (e.g. "lateral_movement")
            - comment: str (optional)
            - score: int (1-100, optional)
            - enabled: bool (default True)
        description: Campaign description

    Returns:
        ATT&CK Navigator layer dict
    """
    if techniques is None:
        techniques = []

    layer = {
        "name": f"{campaign_name} — Vesper Campaign",
        "versions": {
            "attack": "16",
            "navigator": "4.9.0",
            "layer": "4.5",
        },
        "domain": "enterprise-attack",
        "description": description or f"ATT&CK techniques used during campaign '{campaign_name}'",
        "filters": {
            "platforms": ["Linux", "Windows", "macOS", "Network"],
        },
        "sorting": 0,
        "layout": {
            "layout": "side",
            "aggregateFunction": "average",
            "showID": False,
            "showName": True,
            "showAggregateScores": False,
            "countUnscored": False,
            "expandedSubtechniques": "none",
        },
        "hideDisabled": False,
        "techniques": [],
        "gradient": {
            "colors": ["#ff6666", "#ffe766", "#8ec843"],
            "minValue": 1,
            "maxValue": 100,
        },
        "legendItems": [],
        "metadata": [],
    }

    seen_techniques = set()

    for tech in techniques:
        tid = tech.get("technique_id", "")
        if not tid or tid in seen_techniques:
            continue
        seen_techniques.add(tid)

        tactic = tech.get("tactic", "impact")
        score = min(100, max(1, tech.get("score", 50)))
        comment = tech.get("comment", "")

        # Look up technique name if not provided
        tname, ttactic = TECHNIQUE_DB.get(tid, (tid, tactic))
        if tactic == "impact":
            tactic = ttactic

        color = TACTIC_COLORS.get(tactic, "#cccccc")

        entry = {
            "techniqueID": tid,
            "tactic": tactic,
            "color": color,
            "comment": comment,
            "enabled": tech.get("enabled", True),
            "score": score,
            "metadata": [
                {"name": "campaign_id", "value": campaign_id or tech.get("campaign_id", "")},
                {"name": "confidence", "value": str(score)},
            ],
        }

        layer["techniques"].append(entry)

    # Build legend from used tactics
    used_tactics = {}
    for t in layer["techniques"]:
        tactic = t["tactic"]
        if tactic not in used_tactics:
            used_tactics[tactic] = TACTIC_COLORS.get(tactic, "#cccccc")

    for tactic, color in sorted(used_tactics.items()):
        count = sum(1 for t in layer["techniques"] if t["tactic"] == tactic)
        layer["legendItems"].append({
            "label": f"{tactic.replace('_', ' ').title()} ({count})",
            "color": color,
        })

    layer["metadata"] = [
        {"name": "Generated", "value": datetime.now(timezone.utc).isoformat()},
        {"name": "Total Techniques", "value": str(len(layer["techniques"]))},
        {"name": "Campaign", "value": campaign_name},
    ]

    return layer


def generate_from_campaign(decisions: List[Dict], exploits: List[Dict],
                           campaign_name: str = "Vesper",
                           campaign_id: str = "") -> Dict[str, Any]:
    """Generate a Navigator layer from campaign decisions and exploits.

    Args:
        decisions: List of decision dicts with mitre_id, tactic, technique, confidence
        exploits: List of exploit dicts with mitre_id, target, success
        campaign_name: Campaign name
        campaign_id: Campaign ID

    Returns:
        ATT&CK Navigator layer dict
    """
    techniques = []

    for d in decisions:
        if d.get("mitre_id"):
            score = int((d.get("confidence", 0.5)) * 100)
            techniques.append({
                "technique_id": d["mitre_id"],
                "tactic": d.get("tactic", "impact").lower().replace(" ", "_"),
                "score": score,
                "comment": f"Decision: {d.get('technique', '')} — {d.get('reasoning', '')}",
                "campaign_id": campaign_id,
            })

    for e in exploits:
        if e.get("mitre_id"):
            score = 80 if e.get("success") else 30
            techniques.append({
                "technique_id": e["mitre_id"],
                "tactic": "exploitation",
                "score": score,
                "comment": f"Exploit: {e.get('target', '')} — {'success' if e.get('success') else 'failed'}",
                "campaign_id": campaign_id,
            })

    return generate_navigator_layer(
        campaign_name=campaign_name,
        campaign_id=campaign_id,
        techniques=techniques,
    )


def save_layer(layer: Dict[str, Any], path: str = "reports/attack_navigator_layer.json") -> str:
    """Save the Navigator layer to a JSON file.

    Args:
        layer: Navigator layer dict
        path: Output file path

    Returns:
        Absolute path to saved file
    """
    import sys
    sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))
    from safety import resolve_in_lab
    resolved = resolve_in_lab(path)
    resolved.parent.mkdir(parents=True, exist_ok=True)
    with open(resolved, "w") as f:
        json.dump(layer, f, indent=2)
    return str(resolved.resolve())


# ============================================================
# Bridge handler
# ============================================================

def register_routes(registry: dict) -> None:
    registry["reporting"] = {
        "attack_layer": handle_attack_layer,
    }


def handle_attack_layer(params: dict) -> dict:
    """Generate an ATT&CK Navigator layer from campaign data.

    Params:
        campaign_name: Campaign name (default: "Vesper Campaign")
        campaign_id: Campaign ID
        decisions: List of decision dicts (optional)
        exploits: List of exploit dicts (optional)
        technique_ids: List of raw technique IDs (optional, e.g. ["T1210", "T1059"])
        output_path: Where to save the JSON file (default: reports/attack_layer.json)

    Returns:
        dict with path, technique_count, and layer summary
    """
    campaign_name = params.get("campaign_name", "Vesper Campaign")
    campaign_id = params.get("campaign_id", "")
    decisions = params.get("decisions", [])
    exploits = params.get("exploits", [])
    raw_technique_ids = params.get("technique_ids", [])
    output_path = params.get("output_path", "reports/attack_navigator_layer.json")

    techniques = []

    # From decisions
    for d in decisions:
        if isinstance(d, dict) and d.get("mitre_id"):
            score = int(max(1, min(100, d.get("confidence", 0.5) * 100)))
            techniques.append({
                "technique_id": d["mitre_id"],
                "tactic": d.get("tactic", "").lower().replace(" ", "_"),
                "score": score,
                "comment": f"{d.get('technique', '')}: {d.get('reasoning', '')}",
                "campaign_id": campaign_id,
            })

    # From exploits
    for e in exploits:
        if isinstance(e, dict) and e.get("mitre_id"):
            score = 80 if e.get("success", False) else 30
            techniques.append({
                "technique_id": e["mitre_id"],
                "tactic": "exploitation",
                "score": score,
                "comment": f"Exploit on {e.get('target', '?')}: {'success' if e.get('success') else 'failed'}",
                "campaign_id": campaign_id,
            })

    # From raw technique IDs
    for tid in raw_technique_ids:
        if isinstance(tid, str) and tid.startswith("T"):
            tname, tactic = TECHNIQUE_DB.get(tid, (tid, "impact"))
            techniques.append({
                "technique_id": tid,
                "tactic": tactic,
                "score": 70,
                "comment": f"Technique {tid}: {tname}",
                "campaign_id": campaign_id,
            })

    # If no data provided, generate a demo layer showcasing all techniques in DB
    if not techniques:
        for tid, (tname, tactic) in TECHNIQUE_DB.items():
            techniques.append({
                "technique_id": tid,
                "tactic": tactic,
                "score": 50,
                "comment": f"Technique {tid}: {tname} (demo — no campaign data)",
                "campaign_id": campaign_id,
            })

    layer = generate_navigator_layer(
        campaign_name=campaign_name,
        campaign_id=campaign_id,
        techniques=techniques,
    )

    saved_path = save_layer(layer, output_path)

    return {
        "success": True,
        "path": saved_path,
        "technique_count": len(layer["techniques"]),
        "tactics_covered": len(layer["legendItems"]),
        "navigator_url": "https://mitre-attack.github.io/attack-navigator/",
        "layer_name": layer["name"],
        "tactics": [li["label"] for li in layer["legendItems"]],
    }
