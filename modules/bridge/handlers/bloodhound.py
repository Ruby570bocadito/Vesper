# Vesper Bridge — BloodHound / SharpHound Collector
# =================================================
# Collects Active Directory data and maps attack paths.
# Uses SharpHound (Windows) or Python-based BloodHound collector.

import subprocess
from pathlib import Path


def collect_bloodhound(params: dict) -> dict:
    """Collect Active Directory data for BloodHound analysis."""
    domain = params.get("domain", "")
    username = params.get("username", "")
    password = params.get("password", "")
    method = params.get("method", "python")

    results = {
        "domain": domain,
        "collected": False,
        "users": 0,
        "computers": 0,
        "groups": 0,
        "sessions": 0,
        "attack_paths": [],
        "output_file": "",
        "error": "",
    }

    if method == "sharphound":
        results.update(_run_sharphound(domain, username, password))
    elif method == "python":
        results.update(_run_python_collector(domain, username, password))
    else:
        results["error"] = f"Unknown method: {method}"

    if results["collected"] and results["output_file"]:
        results["attack_paths"] = _analyze_paths(results["output_file"])

    return results


def _run_sharphound(domain, username, password) -> dict:
    """Run SharpHound.exe to collect AD data."""
    try:
        cmd = ["SharpHound.exe", "--CollectionMethod", "All", "--Domain", domain]
        if username:
            cmd.extend(["--Username", username, "--Password", password])

        proc = subprocess.run(cmd, capture_output=True, text=True, timeout=300)
        output = proc.stdout + proc.stderr

        # Parse output for stats
        users = _extract_int(output, "Users:")
        computers = _extract_int(output, "Computers:")
        groups = _extract_int(output, "Groups:")

        # Find the zip file
        zip_files = list(Path(".").glob("*BloodHound*.zip"))
        output_file = str(zip_files[0]) if zip_files else ""

        return {
            "collected": bool(output_file),
            "users": users,
            "computers": computers,
            "groups": groups,
            "output_file": output_file,
        }
    except FileNotFoundError:
        return {"error": "SharpHound.exe not found", "collected": False}
    except Exception as e:
        return {"error": str(e), "collected": False}


def _run_python_collector(domain, username, password) -> dict:
    """Run Python-based AD collector."""
    result = {
        "collected": False,
        "users": 0,
        "computers": 0,
        "groups": 0,
        "sessions": 0,
        "output_file": "",
    }

    try:
        # Try to use impacket for LDAP enumeration
        try:
            from impacket.ldap import ldap
            # Enumerate users, computers, groups via LDAP
            result["users"] = 0
            result["computers"] = 0
            result["groups"] = 0
        except ImportError:
            pass

        # Try to enumerate via ldapsearch if available
        try:
            cmd = ["ldapsearch", "-x", "-H", f"ldap://{domain}", "-b", f"DC={domain.replace('.',',DC=')}"]
            proc = subprocess.run(cmd, capture_output=True, text=True, timeout=30)
            if "numEntries" in proc.stdout:
                result["collected"] = True
                result["output_file"] = _save_json_output(proc.stdout, domain)
        except FileNotFoundError:
            pass
        except Exception:
            pass

    except Exception as e:
        result["error"] = str(e)

    return result


def _analyze_paths(output_file: str) -> list:
    """Analyze collected data for attack paths.

    Real path derivation from collected graph data is not implemented
    yet — returning fabricated CORP.LOCAL paths would be fabrication,
    so this reports an empty set until the analyzer lands.
    """
    return []


def _extract_int(text: str, prefix: str) -> int:
    for line in text.splitlines():
        if prefix in line:
            try:
                return int(line.split(prefix)[-1].strip())
            except ValueError:
                pass
    return 0


def _save_json_output(data: str, domain: str) -> str:
    import sys, os
    sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))
    from safety import resolve_in_lab
    output_file = resolve_in_lab(f"tmp/bloodhound_{domain.replace('.','_')}.json")
    output_file.parent.mkdir(parents=True, exist_ok=True)
    with open(output_file, "w") as f:
        f.write(data)
    return str(output_file)


def register_routes(registry: dict) -> None:
    """Expose this module's handlers under the 'bloodhound' group."""
    registry["bloodhound"] = {
        "collect": collect_bloodhound,
    }
