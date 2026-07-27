# Operations

## Build

```sh
make dist
```

Output: `dist/com.lalbers.community-admin-<version>.tar.gz`

## Install (production)

Installation instructions are in the [root README](../README.md#installation). The recommended approach is:

- **Release tarball** — download from [Releases](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/releases) and install via `mmctl plugin add` (see the [Configuration guide](configuration.md) for prerequisites)
- **Build from source** — `make dist` then `mmctl plugin add dist/com.lalbers.community-admin-*.tar.gz`
- **mattermost-oci-deploy** — if using this deployment helper, run `scripts/install-community-admin-plugin.sh`

## Test instance (mattermost-oci-deploy)

| Item | Value |
|------|-------|
| URL | `https://<test-hostname>` |
| VM | `ubuntu@<test-ip>` |
| Start / stop | `/opt/mattermost/ops/manage-test-instance.sh start\|stop` |
| Access | IP-restricted (`TEST_ALLOWED_CIDR` in deploy `generated.env`) |

After starting the test container, confirm the plugin is active:

```sh
mmctl --local plugin list
curl -sS https://<test-hostname>/api/v4/plugins/webapp -H "Authorization: Bearer ***"
```

## Deploy updated plugin to test

From your workstation (plugin repo):

```sh
make dist
scp dist/com.lalbers.community-admin-*.tar.gz ubuntu@<test-ip>:/tmp/community-admin.tar.gz
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

## Upgrading

To upgrade an existing installation:

1. Download the newer `com.lalbers.community-admin-*.tar.gz` from [Releases](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/releases).
2. On the Mattermost server:
   ```sh
   mmctl --local plugin disable com.lalbers.community-admin
   mmctl --local plugin delete com.lalbers.community-admin
   mmctl --local plugin add /path/to/com.lalbers.community-admin-<version>.tar.gz
   mmctl --local plugin enable com.lalbers.community-admin
   ```
3. Hard-refresh the browser after webapp updates (bundle is cached).

After upgrading, confirm the plugin is active:

```sh
mmctl --local plugin list | grep community-admin
```

If the panel does not load after upgrade, see [Webapp bundle 404](#webapp-bundle-404) below.

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
