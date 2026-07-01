/**
 * StarCrystal H5 — 激励广告桥接（UniWebView / CrystaBoxAdManager）
 * 各小游戏通过 placement 区分埋点：g001 / g002 / g003
 */
(function (global) {
  function setStatus(el, text, tone) {
    if (!el) return;
    el.textContent = text || '';
    el.style.color = tone === 'ok' ? '#7dffb0' : tone === 'err' ? '#ff9a9a' : '#a8d4b8';
  }

  function showRewarded(opts) {
    var placement = (opts && opts.placement) || 'h5';
    var prefix = (opts && opts.requestIdPrefix) || placement;
    var rid = prefix + '-' + Date.now();
    if (global.CrystaBoxAdManager && typeof global.CrystaBoxAdManager.showRewarded === 'function') {
      global.CrystaBoxAdManager.showRewarded({ requestId: rid, placement: placement });
      return true;
    }
    try {
      var q = 'uniwebview://crystabox/h5?action=' + encodeURIComponent('rewarded.show')
        + '&rid=' + encodeURIComponent(rid)
        + '&placement=' + encodeURIComponent(placement);
      global.location.href = q;
      return true;
    } catch (e) {
      return false;
    }
  }

  function bindRewardedUI(btn, statusEl, placement, onSuccess) {
    var busy = false;
    function finish(d) {
      busy = false;
      if (btn) btn.disabled = false;
      var ok = d && d.success;
      var line = ok
        ? ('激励完成 · ' + (d.code || '') + (d.message ? ' · ' + d.message : ''))
        : ('激励未达成 · ' + (d.code || '') + (d.message ? ' · ' + d.message : ''));
      setStatus(statusEl, line, ok ? 'ok' : 'err');
      if (ok && typeof onSuccess === 'function') onSuccess(d);
    }
    global.addEventListener('CrystaBoxNativeAd', function (ev) {
      var d = ev && ev.detail;
      if (d) finish(d);
    });
    global.onCrystaBoxNativeAd = function (d) {
      if (d) finish(d);
    };
    btn.addEventListener('click', function () {
      if (busy) return;
      if (!showRewarded({ placement: placement, requestIdPrefix: placement })) {
        setStatus(statusEl, '无法调用广告桥（请在 App 内 WebView 打开）。', 'err');
        return;
      }
      busy = true;
      btn.disabled = true;
      setStatus(statusEl, '已向宿主请求激励广告…', 'muted');
      setTimeout(function () {
        busy = false;
        btn.disabled = false;
      }, 120000);
    });
  }

  global.CrystaBoxH5Ad = {
    showRewarded: showRewarded,
    setStatus: setStatus,
    bindRewardedUI: bindRewardedUI
  };
})(typeof window !== 'undefined' ? window : this);
