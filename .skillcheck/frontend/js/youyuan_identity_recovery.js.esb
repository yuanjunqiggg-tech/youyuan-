(function() {
  function recovered(user) {
    if (!yyIsCloud || !(user == null ? void 0 : user.cloud_allowed)) return;
    window.yyCloudBlocked = false;
    if (window.yyLinuxTransportStatus) {
      window.yyLinuxTransportStatus.auth_required = false;
      if (window.yyLinuxTransportStatus.last_error === "\u767B\u5F55\u72B6\u6001\u5DF2\u5931\u6548\uFF0C\u8BF7\u91CD\u65B0\u767B\u5F55") window.yyLinuxTransportStatus.last_error = "";
    }
  }
  const base = yyIdentityReady;
  yyIdentityReady = function(user) {
    recovered(user);
    return base.apply(this, arguments);
  };
  document.addEventListener("yy-identity-ready", () => recovered(yyPlatformUser));
  window.addEventListener("yy-cloud-auth-expired", () => {
    if (!yyIsCloud) return;
    let panel = document.getElementById("yy-identity-gate");
    if (!panel || !document.getElementById("yy-login-name")) {
      yyShowIdentity();
      panel = document.getElementById("yy-identity-gate");
    }
    if (panel) {
      panel.style.zIndex = "2147483000";
      panel.setAttribute("aria-label", "\u91CD\u65B0\u767B\u5F55\u6E38\u56ED");
    }
    const result = document.getElementById("yy-login-result");
    if (result && !result.textContent) result.textContent = "\u767B\u5F55\u72B6\u6001\u5DF2\u5931\u6548\uFF0C\u8BF7\u91CD\u65B0\u767B\u5F55\u3002\u8BBE\u5907\u4E0A\u7684\u673A\u5668\u4EBA\u3001\u6587\u4EF6\u4E0E\u4EFB\u52A1\u4F1A\u4FDD\u7559\u3002";
  });
})();
