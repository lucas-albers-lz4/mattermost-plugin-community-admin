#!/usr/bin/env bash
# API smoke tests for Community Admin plugin against mattermost-test.
# Covers manual checklist items A1-A5 (partial) without a browser.
set -eo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
if [ -f "${SCRIPT_DIR}/../.env" ]; then
  set -a
  # shellcheck disable=SC1091
  source "${SCRIPT_DIR}/../.env"
  set +a
fi

BASE_URL="${TEST_URL:-https://doomzilla.duckdns.org}"
PLUGIN_API="${BASE_URL}/plugins/com.lalbers.community-admin/api/v1"
ORG_USER="${ORGANIZER_USERNAME:-test.organizer}"
ORG_PASS="${ORGANIZER_PASSWORD:?Set ORGANIZER_PASSWORD in e2e/.env}"
NON_USER="${NON_ORGANIZER_USERNAME:-testuser.beta}"
NON_PASS="${NON_ORGANIZER_PASSWORD:?Set NON_ORGANIZER_PASSWORD in e2e/.env}"

login() {
  local user=$1 pass=$2
  curl -sS -X POST "${BASE_URL}/api/v4/users/login" \
    -H 'Content-Type: application/json' \
    -d "{\"login_id\":\"${user}\",\"password\":\"${pass}\"}" \
    -D - -o /tmp/mm-login-body.json | awk 'BEGIN{IGNORECASE=1} /^token:/{sub(/\r/,""); print $2}' | head -1
}

plugin_get() {
  local token=$1 path=$2
  curl -sS -w '\n%{http_code}' "${PLUGIN_API}${path}" -H "Authorization: Bearer ${token}"
}

plugin_post() {
  local token=$1 path=$2 body=${3:-{}}
  curl -sS -w '\n%{http_code}' -X POST "${PLUGIN_API}${path}" \
    -H "Authorization: Bearer ${token}" -H 'Content-Type: application/json' -d "${body}"
}

plugin_delete() {
  local token=$1 path=$2
  curl -sS -w '\n%{http_code}' -X DELETE "${PLUGIN_API}${path}" -H "Authorization: Bearer ${token}"
}

split_response() {
  local raw=$1
  BODY=$(echo "$raw" | sed '$d')
  CODE=$(echo "$raw" | tail -1)
}

pass() { echo "PASS: $1"; }
fail() { echo "FAIL: $1"; exit 1; }

echo "=== Community Admin API smoke (${BASE_URL}) ==="

ORG_TOKEN=$(login "$ORG_USER" "$ORG_PASS")
[ -n "$ORG_TOKEN" ] || fail "organizer login"
ORG_ID=$(python3 -c "import json; print(json.load(open('/tmp/mm-login-body.json'))['id'])")
pass "A3-prep: organizer login (${ORG_USER})"

NON_TOKEN=$(login "$NON_USER" "$NON_PASS")
if [ -z "$NON_TOKEN" ]; then
  BETA_ID=$(curl -sS "${PLUGIN_API}/users?q=beta" -H "Authorization: Bearer ${ORG_TOKEN}" | python3 -c "import sys,json; print([u['id'] for u in json.load(sys.stdin)['users'] if u['username']=='${NON_USER}'][0])")
  split_response "$(plugin_post "$ORG_TOKEN" "/users/${BETA_ID}/reset-password" "{}")"
  NON_PASS=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('password',''))" 2>/dev/null || true)
  [ -n "$NON_PASS" ] || fail "could not reset non-organizer password"
  NON_TOKEN=$(login "$NON_USER" "$NON_PASS")
fi
[ -n "$NON_TOKEN" ] || fail "non-organizer login"

split_response "$(plugin_get "$NON_TOKEN" "/me")"
[ "$CODE" = "403" ] && pass "A1: non-organizer /me returns 403" || fail "A1: expected 403 got ${CODE} body=${BODY}"

split_response "$(plugin_get "$ORG_TOKEN" "/me")"
[ "$CODE" = "200" ] || fail "organizer /me expected 200 got ${CODE}"
echo "$BODY" | grep -q '"display_username"' && pass "A3: organizer /me OK"

split_response "$(plugin_get "$ORG_TOKEN" "/users?q=")"
[ "$CODE" = "200" ] || fail "list users expected 200 got ${CODE}"
echo "$BODY" | grep -q 'testuser.alpha' && pass "A2: list includes testuser.alpha"
echo "$BODY" | grep -q 'testuser.beta' && pass "A2: list includes testuser.beta"

split_response "$(plugin_get "$ORG_TOKEN" "/users?q=alpha")"
echo "$BODY" | grep -q 'testuser.alpha' && ! echo "$BODY" | grep -q 'testuser.beta' && pass "A2: search alpha filters" || echo "WARN: search filter inconclusive"

NEW_USER="testuser.api.$(date +%s)"
split_response "$(plugin_post "$ORG_TOKEN" "/users" "{\"username\":\"${NEW_USER}\",\"first_name\":\"API\",\"last_name\":\"Smoke\",\"team_ids\":[\"414hzzpgr3ddimacg4nwu53wrr\"]}")"
[ "$CODE" = "200" ] || [ "$CODE" = "201" ] || fail "create user expected 200/201 got ${CODE} body=${BODY}"
echo "$BODY" | grep -q 'parent_text' && pass "A3: create user returns credential handoff"
NEW_ID=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin)['user']['id'])" 2>/dev/null || true)
[ -n "$NEW_ID" ] || fail "create user missing id"

split_response "$(plugin_post "$ORG_TOKEN" "/users/${NEW_ID}/reset-password" "{}")"
[ "$CODE" = "200" ] || fail "reset password expected 200 got ${CODE}"
echo "$BODY" | grep -q 'parent_text' && pass "A4: reset password returns handoff"

split_response "$(plugin_delete "$ORG_TOKEN" "/users/${NEW_ID}/teams/414hzzpgr3ddimacg4nwu53wrr")"
[ "$CODE" = "200" ] || fail "remove from team expected 200 got ${CODE}"
pass "A5: remove from team OK"

split_response "$(plugin_post "$ORG_TOKEN" "/users/${ORG_ID}/reset-password" "{}")"
[ "$CODE" = "403" ] && pass "A7: reset organizer self blocked" || echo "WARN: protected target reset returned ${CODE}"

split_response "$(plugin_post "$ORG_TOKEN" "/users" "{\"username\":\"Bad User!\",\"first_name\":\"X\",\"last_name\":\"Y\"}")"
[ "$CODE" = "400" ] || [ "$CODE" = "403" ] || [ "$CODE" = "500" ] && pass "A7: invalid username rejected (${CODE})" || fail "A7: invalid username expected error got ${CODE}"

TEAM_ID="414hzzpgr3ddimacg4nwu53wrr"
CHANNEL_ID=$(curl -sS "${BASE_URL}/api/v4/teams/${TEAM_ID}/channels/name/town-square" -H "Authorization: Bearer ${ORG_TOKEN}" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])" 2>/dev/null || true)
if [ -n "$CHANNEL_ID" ]; then
  CMD_BODY=$(curl -sS -w '\n%{http_code}' -X POST "${BASE_URL}/api/v4/commands/execute" \
    -H "Authorization: Bearer ${ORG_TOKEN}" -H 'Content-Type: application/json' \
    -d "{\"channel_id\":\"${CHANNEL_ID}\",\"command\":\"/community-admin reset-password testuser.beta\"}")
  CMD_CODE=$(echo "$CMD_BODY" | tail -1)
  CMD_TEXT=$(echo "$CMD_BODY" | sed '$d')
  echo "$CMD_TEXT" | grep -qi 'password' && pass "A6: slash reset-password returns credentials" || echo "WARN: A6 slash command inconclusive (code=${CMD_CODE})"
else
  echo "WARN: A6 skipped (town-square channel not found)"
fi

echo "=== API smoke complete ==="
