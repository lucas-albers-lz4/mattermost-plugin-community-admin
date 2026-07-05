# Operations

## Build

```sh
make dist
```

Output: `dist/com.lalbers.community-admin-<version>.tar.gz`

## Install (production)

From the [mattermost-oci-deploy](https://github.com/lalbers/mattermost-oci-deploy) repository (optional helper for a specific deployment):

```sh
scripts/install-community-admin-plugin.sh
```

Or manually inside the Mattermost container:

```sh
mmctl --local plugin add /path/to/com.lalbers.community-admin.tar.gz
mmctl --local plugin enable com.lalbers.community-admin
```

`PluginSettings.EnableUploads` must be true for `plugin add` unless the tarball is placed directly in the configured plugin directory by an operator.

## Test instance (mattermost-oci-deploy)

| Item | Value |
|------|-------|
| URL | `https://doomzilla.duckdns.org` |
| VM | `ubuntu@129.146.67.228` |
| Start / stop | `/opt/mattermost/ops/manage-test-instance.sh start\|stop` |
| Access | IP-restricted (`TEST_ALLOWED_CIDR` in deploy `generated.env`) |

After starting the test container, confirm the plugin is active:

```sh
mmctl --local plugin list
curl -sS https://doomzilla.duckdns.org/api/v4/plugins/webapp -H "Authorization: Bearer <token>"
```

## Deploy updated plugin to test

From your workstation (plugin repo):

```sh
make dist
scp dist/com.lalbers.community-admin-*.tar.gz ubuntu@129.146.67.228:/tmp/community-admin.tar.gz
```

On the VM:

```sh
docker compose --env-file /opt/mattermost/.env -p mattermost -f /opt/mattermost/compose.yml \
  --profile upgrade-test cp /tmp/community-admin.tar.gz mattermost-test:/tmp/community-admin.tar.gz

docker compose ... exec -T mattermost-test sh -c '
  mmctl plugin disable com.lalbers.community-admin --local 2>/dev/null || true
  mmctl plugin delete com.lalbers.community-admin --local 2>/dev/null || true
  mmctl plugin add /tmp/community-admin.tar.gz --local
  mmctl plugin enable com.lalbers.community-admin --local
'
```

Hard-refresh the browser after webapp updates (bundle is cached).

## Troubleshooting

### Webapp bundle 404

Mattermost advertises bundle URLs at `/static/com.lalbers.community-admin/<hash>_bundle.js` but may only serve files under `/static/plugins/...`. If the panel never loads and the browser network tab shows 404 on the bundle:

```sh
# Inside mattermost-test container
BUNDLE=$(ls /mattermost/client/plugins/com.lalbers.community-admin/*bundle.js | head -1)
mkdir -p /mattermost/client/com.lalbers.community-admin
cp "$BUNDLE" /mattermost/client/com.lalbers.community-admin/$(basename "$BUNDLE")
```

### Plugin missing after container restart

Do not rely on copying tarballs only into `/mattermost/plugins/`. Mattermost removes unmanaged local installs on startup. Always install with `mmctl plugin add` (requires `EnableUploads`) so the plugin is tracked in the database.

### Community Members not in menus (MM 11 Entry)

The plugin registers menu and channel-header actions in Redux. On Entry Edition the product switch menu may omit plugin main-menu items. Use the **channel header** button or see [user-guide.md](user-guide.md).

### Password reset fails

Confirm `ServiceSettings.EnableLocalMode: true` and that `/mattermost/bin/mmctl` exists in the container.
