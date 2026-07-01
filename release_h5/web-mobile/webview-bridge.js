(function () {
    if (window.AppWebViewBridge) {
        return;
    }

    function safeStringify(payload) {
        try {
            return JSON.stringify(payload || {});
        } catch (e) {
            return "{}";
        }
    }

    function postToNative(payload) {
        var body = safeStringify(payload);

        try {
            if (window.webkit && window.webkit.messageHandlers && window.webkit.messageHandlers.gameBridge) {
                window.webkit.messageHandlers.gameBridge.postMessage(payload || {});
                return true;
            }
        } catch (e1) {}

        try {
            if (window.AndroidBridge && typeof window.AndroidBridge.postMessage === "function") {
                window.AndroidBridge.postMessage(body);
                return true;
            }
        } catch (e2) {}

        try {
            if (window.ReactNativeWebView && typeof window.ReactNativeWebView.postMessage === "function") {
                window.ReactNativeWebView.postMessage(body);
                return true;
            }
        } catch (e3) {}

        return false;
    }

    window.AppWebViewBridge = {
        postMessage: function (eventName, data) {
            return postToNative({
                event: eventName || "message",
                data: data || {},
                ts: Date.now()
            });
        },
        onNativeMessage: function (handler) {
            if (typeof handler !== "function") {
                return;
            }
            window.__onNativeMessage = handler;
        }
    };

    window.dispatchNativeToGame = function (raw) {
        var data = raw;
        if (typeof raw === "string") {
            try {
                data = JSON.parse(raw);
            } catch (e) {
                data = { event: "raw", data: raw };
            }
        }
        if (typeof window.__onNativeMessage === "function") {
            window.__onNativeMessage(data || {});
        }
    };

    postToNative({
        event: "game_ready",
        data: { ua: navigator.userAgent },
        ts: Date.now()
    });
})();
