#!/usr/bin/env bash
set -euo pipefail

discord_id=$(
  curl -fsS --max-time 10 "${SENTINEL_URL%/}/core/entity/external/GITHUB/${GITHUB_USER_ID}" \
    -H "Authorization: Bearer ${TOKEN}" \
    | jq -r '[.external_auths[]? | select(.provider == "DISCORD") | .external_id][0] // empty'
) || {
  echo "::warning::Could not look up a Discord account for GitHub user ${GITHUB_LOGIN}."
  discord_id=""
}

if [ -n "$discord_id" ]; then
  who="<@${discord_id}>"
else
  who="**${GITHUB_LOGIN}**"
fi

target="**${SERVICE}${VERSION:+ ${VERSION}}**"
pr_label=$(sed -E 's#^https://github.com/[^/]+/([^/]+)/pull/([0-9]+).*#\1\#\2#' <<<"$PR_URL")
content="${who} wants to deploy ${target}! [${pr_label}](<${PR_URL}>)"

jq -n --arg channel "$CHANNEL_ID" --arg content "$content" '{
  channel: "DISCORD",
  category: "deploy",
  recipients: { entity_ids: [], group_ids: [], discord_channel_ids: [$channel] },
  discord: {
    content: $content,
    webhook: { username: "Deploys", avatar_url: "https://gauchoracing.com/logo/gr-logo.png" }
  }
}' | curl -fsS --max-time 15 -X POST "${NOTIFY_URL%/}/messages" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --data @-
echo
