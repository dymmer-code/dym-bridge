# Releases

A tag `vX.Y.Z` builds every system and publishes the archives and `checksums.txt`.

## macOS: signing and notarization

macOS only opens an app downloaded from the internet if it is signed with a Developer ID and notarized by Apple.
When the five secrets below exist in the repository, the macOS jobs sign the app, put it in a `.dmg`, notarize the
disk image, staple the ticket and publish the `.dmg` instead of the `.tar.gz`. Without them the archive is
published unsigned and the job prints a warning.

| Secret | Content |
|---|---|
| `MACOS_CERTIFICATE_P12` | the *Developer ID Application* certificate with its private key, exported as `.p12`, in base64 |
| `MACOS_CERTIFICATE_PASSWORD` | the password chosen when exporting the `.p12` |
| `MACOS_NOTARY_KEY` | the App Store Connect API key (`AuthKey_XXXX.p8`) in base64 |
| `MACOS_NOTARY_KEY_ID` | the key id of that API key |
| `MACOS_NOTARY_ISSUER_ID` | the issuer id shown in App Store Connect, Users and Access, Integrations |

The `.p12` and the key are never printed and are deleted at the end of the job.
