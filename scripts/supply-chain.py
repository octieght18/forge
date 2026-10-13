#!/usr/bin/env python3
"""Module SBOM, with image scan and signing only when the tool and image exist.

A signature shows that an artifact matches an operator-held key. It does not
prove the artifact is safe. This script never prints a clean-scan result unless
a scanner process actually ran.
"""
import argparse
import json
import os
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path


def module_components(go_sum):
    found = {}
    for line in go_sum.splitlines():
        parts = line.split()
        if len(parts) < 2 or parts[1].endswith("/go.mod"):
            continue
        name, version = parts[0], parts[1]
        found[(name, version)] = {
            "type": "library",
            "name": name,
            "version": version,
            "purl": f"pkg:golang/{name}@{version}",
        }
    return [found[key] for key in sorted(found)]


def module_sbom(components):
    return {
        "bomFormat": "CycloneDX",
        "specVersion": "1.5",
        "version": 1,
        "metadata": {
            "component": {
                "type": "application",
                "name": "github.com/octieght18/forge",
                "version": "unreleased",
            }
        },
        "components": components,
    }


def plan(found, image, key):
    """Decide which external tools can run. Absence is not a clean scan."""
    scan_tools = [name for name in ("grype", "trivy") if name in found]
    return {
        "module_sbom": "run",
        "image_sbom": "run" if "syft" in found and image else "not-run",
        "image_scan": "run" if scan_tools and image else "not-run",
        "signed_image": "run" if "cosign" in found and image and key else "not-run",
        "clean_scan": "not-claimed",
        "signature_meaning": "matches a key, does not prove safety",
    }


def main(argv):
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true")
    parser.add_argument("--go-sum", default="go.sum")
    parser.add_argument("--image", default=os.environ.get("FORGE_IMAGE", ""))
    parser.add_argument("--cosign-key", default=os.environ.get("FORGE_COSIGN_KEY", ""))
    args = parser.parse_args(argv)
    components = module_components(Path(args.go_sum).read_text(encoding="utf-8"))
    if not components:
        print("module SBOM is empty", file=sys.stderr)
        return 1
    document = module_sbom(components)
    if args.check:
        handle = tempfile.NamedTemporaryFile(prefix="forge-module-", suffix=".cdx.json", delete=False)
        with handle:
            handle.write(json.dumps(document).encode())
        print(f"module-sbom components={len(components)} path={handle.name}")
    found = {name for name in ("syft", "grype", "trivy", "cosign") if shutil.which(name)}
    for name in ("syft", "grype", "trivy", "cosign"):
        print(("present" if name in found else "absent") + f": {name}")
    decision = plan(found, args.image, args.cosign_key)
    print("image-sbom: " + decision["image_sbom"])
    print("image-scan: " + decision["image_scan"])
    print("signed-image: " + decision["signed_image"])
    print("clean-scan: " + decision["clean_scan"])
    print("signature-meaning: " + decision["signature_meaning"])
    if decision["image_sbom"] == "run":
        subprocess.run(["syft", f"image:{args.image}", "-o", "cyclonedx-json"], check=True)
    if decision["image_scan"] == "run":
        if "grype" in found:
            subprocess.run(["grype", args.image], check=False)
        if "trivy" in found:
            subprocess.run(["trivy", "image", "--exit-code", "0", args.image], check=False)
    if decision["signed_image"] == "run":
        subprocess.run(["cosign", "sign", "--key", args.cosign_key, args.image], check=True)
        subprocess.run(["cosign", "verify", "--key", args.cosign_key, args.image], check=True)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
