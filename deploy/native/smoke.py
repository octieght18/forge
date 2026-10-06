#!/usr/bin/env python3
"""Real native Keycloak/PKCE + Forge smoke; synthetic credentials stay in memory."""
import argparse
import base64
import hashlib
from html.parser import HTMLParser
import http.cookiejar
import json
from pathlib import Path
import secrets
import subprocess
import time
import urllib.error
import urllib.parse
import urllib.request

REPO = Path(__file__).resolve().parents[2]
ISSUER = "http://127.0.0.1:8082/realms/forge"
API = "http://127.0.0.1:8081"
CALLBACK = "http://127.0.0.1:8083/callback"


class Form(HTMLParser):
    def __init__(self):
        super().__init__()
        self.action = None
        self.fields = {}
        self.inside = False

    def handle_starttag(self, tag, attrs):
        attrs = dict(attrs)
        if tag == "form" and attrs.get("id") == "kc-form-login":
            self.action = attrs["action"]
            self.inside = True
        if tag == "input" and self.inside and attrs.get("type") == "hidden":
            self.fields[attrs["name"]] = attrs.get("value", "")

    def handle_endtag(self, tag):
        if tag == "form":
            self.inside = False


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, request, fp, code, msg, headers, url):
        return None


class LoopbackCookiePolicy(http.cookiejar.DefaultCookiePolicy):
    def return_ok_secure(self, cookie, request):
        # Browsers trust loopback origins; Keycloak therefore marks these cookies
        # Secure. urllib does not implement the browser's localhost exception.
        if urllib.parse.urlparse(request.full_url).hostname == "127.0.0.1":
            return True
        return super().return_ok_secure(cookie, request)


def request(opener, url, data=None, headers=None):
    try:
        response = opener.open(urllib.request.Request(url, data=data, headers=headers or {}), timeout=10)
        return response.status, response.headers, response.read()
    except urllib.error.HTTPError as error:
        return error.code, error.headers, error.read()


def submit_login(opener, authorization, username, password):
    status, _, page = request(opener, authorization)
    if status != 200:
        raise RuntimeError("Authorization page unavailable")
    form = Form()
    form.feed(page.decode())
    if not form.action or not form.action.startswith(ISSUER + "/login-actions/"):
        raise RuntimeError("Unexpected identity form")
    form.fields.update(username=username, password=password)
    return request(opener, form.action, urllib.parse.urlencode(form.fields).encode())


def login(root, username, password):
    token_file = root / "tokens" / (username + ".json")
    if token_file.exists():
        token_file.unlink()
    child = subprocess.Popen([str(root / "bin/forge-login"), "--token-file", str(token_file)], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    try:
        deadline = time.monotonic() + 15
        opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar(policy=LoopbackCookiePolicy())))
        while time.monotonic() < deadline:
            try:
                request(opener, "http://127.0.0.1:8083/")
                break
            except urllib.error.URLError:
                time.sleep(.2)
        # The helper constructs S256/state/cookie, exchanges the real authorization
        # code, validates the signed access token, and writes the private token file.
        status, _, result = submit_login(opener, "http://127.0.0.1:8083/login", username, password)
        if status != 200 or b"Login complete" not in result or child.wait(timeout=10) != 0:
            raise RuntimeError("Real PKCE helper login failed for " + username)
        token = json.loads(token_file.read_text())
        if token_file.stat().st_mode & 0o077:
            raise RuntimeError("Token file permissions are not private")
        return token
    finally:
        if child.poll() is None:
            child.terminate()
            child.wait(timeout=10)


def negative_pkce(values):
    opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar(policy=LoopbackCookiePolicy())), NoRedirect())
    verifier = secrets.token_urlsafe(32)
    challenge = base64.urlsafe_b64encode(hashlib.sha256(verifier.encode()).digest()).decode().rstrip("=")
    params = dict(client_id="forge-local-login", response_type="code", scope="openid", redirect_uri=CALLBACK,
                  state=secrets.token_hex(16), code_challenge=challenge, code_challenge_method="S256")
    status, headers, _ = submit_login(opener, ISSUER + "/protocol/openid-connect/auth?" + urllib.parse.urlencode(params), "ahmad", values["ahmad"])
    if status != 302 or not headers.get("Location", "").startswith(CALLBACK):
        raise RuntimeError("Real authorization code was not issued")
    code = urllib.parse.parse_qs(urllib.parse.urlparse(headers["Location"]).query)["code"][0]
    body = dict(grant_type="authorization_code", client_id="forge-local-login", redirect_uri=CALLBACK, code=code, code_verifier=secrets.token_urlsafe(32))
    status, _, payload = request(opener, ISSUER + "/protocol/openid-connect/token", urllib.parse.urlencode(body).encode())
    if status != 400 or json.loads(payload).get("error") != "invalid_grant":
        raise RuntimeError("Incorrect PKCE verifier accepted")
    del params["code_challenge"]
    del params["code_challenge_method"]
    status, headers, _ = request(opener, ISSUER + "/protocol/openid-connect/auth?" + urllib.parse.urlencode(params))
    if status != 400 and "error=" not in headers.get("Location", ""):
        raise RuntimeError("Missing PKCE accepted")
    body = dict(grant_type="password", client_id="forge-local-login", username="ahmad", password=values["ahmad"])
    status, _, payload = request(opener, ISSUER + "/protocol/openid-connect/token", urllib.parse.urlencode(body).encode())
    if status != 400 or json.loads(payload).get("error") != "unauthorized_client":
        raise RuntimeError("Password grant unexpectedly enabled")


def api(method, path, token=None, data=None, extra=None, expected=200):
    headers = dict(extra or {})
    if token:
        headers["Authorization"] = "Bearer " + token["access_token"]
    if data is not None:
        headers["Content-Type"] = "application/json"
    req = urllib.request.Request(API + path, method=method, data=json.dumps(data).encode() if data is not None else None, headers=headers)
    try:
        response = urllib.request.urlopen(req, timeout=10)
    except urllib.error.HTTPError as error:
        response = error
    payload = json.loads(response.read())
    if response.code != expected or not response.headers.get("X-Request-ID"):
        raise RuntimeError(f"Unexpected API response for {method} {path}: {response.code}, expected {expected}")
    return payload, response.headers


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("phase", choices=("seed", "verify"))
    p.add_argument("--state-dir", default=str(Path.home() / ".local/share/forge-native"))
    options = p.parse_args()
    root = Path(options.state_dir)
    values = json.loads((root / "secrets.json").read_text())
    tokens = {name: login(root, name, values[name]) for name in ("ahmad", "second-owner", "operator")}
    if tokens["operator"]["role"] != "operator" or tokens["ahmad"]["role"] != "developer" or tokens["second-owner"]["role"] != "developer":
        raise RuntimeError("Real client role mapping failed")
    negative_pkce(values)
    proof_file = root / "validation-proof.json"
    if options.phase == "seed":
        workload, _ = api("POST", "/api/v1/workloads", tokens["ahmad"], {"name": "f07-smoke-" + secrets.token_hex(6), "description": "Native deployment validation"}, expected=201)
        version, _ = api("POST", "/api/v1/workloads/" + workload["workload_id"] + "/versions", tokens["ahmad"], json.loads((REPO / "internal/contract/examples/create-version.json").read_text()), expected=201)
        proof = dict(workload_id=workload["workload_id"], version_id=version["version_id"], subjects={k: v["subject"] for k, v in tokens.items()}, cursor_hash=hashlib.sha256((root / "cursor.key").read_bytes()).hexdigest())
        proof_file.write_text(json.dumps(proof))
        proof_file.chmod(0o600)
    else:
        proof = json.loads(proof_file.read_text())
        if proof["subjects"] != {k: v["subject"] for k, v in tokens.items()} or proof["cursor_hash"] != hashlib.sha256((root / "cursor.key").read_bytes()).hexdigest():
            raise RuntimeError("Identity or cursor key changed on restart")
    path = "/api/v1/workloads/" + proof["workload_id"]
    api("GET", path, tokens["ahmad"])
    api("GET", path + "/versions/" + proof["version_id"], tokens["ahmad"])
    api("GET", path, tokens["operator"])
    api("PATCH", path, tokens["operator"], {"name": "forbidden", "description": "forbidden"}, expected=404)
    api("GET", path, tokens["second-owner"], expected=404)
    api("PATCH", path, tokens["second-owner"], {"name": "forbidden", "description": "forbidden"}, expected=404)
    api("GET", path, expected=401)
    print(json.dumps(dict(phase=options.phase, real_pkce=True, wrong_and_missing_pkce_rejected=True, password_grant_disabled=True,
                         owner_isolation=True, operator_inspection=True, immutable_version_read=True, retained_identity_and_key=options.phase == "verify")))


if __name__ == "__main__":
    main()
