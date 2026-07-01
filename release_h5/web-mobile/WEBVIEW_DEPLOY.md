# WebView Deployment Notes

This build is ready to run in an app WebView.

## Entry

- **Do not open `index.html` with `file://` in a desktop browser** — the game will stay on the loading splash, because the engine loads bundle assets via XHR, which is blocked for `file://` URLs.
- For local testing: run `serve-web-mobile.cmd` in this folder, then open `http://127.0.0.1:8765`.
- In an in-app WebView, load the same content over `http://` or `https://` (or your platform’s packaged asset URL), not a raw `file://` path in Chrome unless you add special flags (not recommended).

## Native -> Game

- Call global JS method:
  - `dispatchNativeToGame(payload)`
- `payload` can be a JSON string or object.

## Game -> Native

- Use global JS object:
  - `AppWebViewBridge.postMessage(eventName, data)`
- The bridge auto-detects one of:
  - `window.webkit.messageHandlers.gameBridge.postMessage` (iOS WKWebView)
  - `window.AndroidBridge.postMessage` (Android custom bridge)
  - `window.ReactNativeWebView.postMessage` (React Native WebView)

## Ready Event

- On load, game sends:
  - `{ event: "game_ready", data: { ua }, ts }`

## App-side recommendation

- Enable JS.
- Allow media auto-play if needed by product.
- Keep hardware acceleration enabled.
- If using local files, ensure WebView can read local assets.
