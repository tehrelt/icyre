Accessible range control built on a native input with a custom track.

**Provide:** `label` (required), `value`/`onChange` or `defaultValue`, `min`, `max`, `step`, `tone` (`neutral` | `iridescent`), `size`, `buffered`, `valueText`, `showThumb` (`always` | `hover`).

- The fill spans the full track and is clipped to the value, so the spectrum stays anchored to the track instead of squeezing.
- `iridescent` is reserved for playback position (ProgressBar). Everything else — volume, crossfade, EQ — is `neutral`.
