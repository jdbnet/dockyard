# Updates

Dockyard compares **registry digests** to the digests recorded on your local image (`RepoDigests`). The default **scan interval** is `updates.interval` (1 hour). Change it in Settings; it applies after a restart.

Use **Scan now** on the Updates page to refresh immediately. **Update** recreates one container; **Update all** runs them in sequence and offers to remove old images afterwards.

Registry checks use a **HEAD** request (not a full manifest pull) so frequent checks stay within Docker Hub limits. Private registries use the same credentials as `docker login` (`~/.docker/config.json` for the Dockyard user).

## Auto-update

In **Settings → General**:

- **Auto-update all containers** — recreate containers when a newer image is found (applies immediately).
- **Auto-update check interval** — how often to check; leave blank to use the scan interval.

Per-container labels override the global toggle. See [Labels](/help/labels).

Behaviour:

- Minimum check interval **10s** (`dockyard.interval` or settings).
- Overlap protection: only one update per container name at a time.
- Dockyard **never** auto-updates itself.
- Failed updates back off until the remote digest changes.
- Old images are removed automatically when unused after an auto-update.
