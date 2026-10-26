/* 旧内核兼容层 —— 必须最先加载，早于任何业务脚本。
 *
 * 为什么需要：esbuild 只能把「语法」降级到 ES2019（?. ?? .at），
 * 但 replaceChildren / Object.hasOwn / replaceAll 是「运行时 API」，
 * 语法降级不会替换它们。旧 Android WebView 与国产内核上这些方法不存在，
 * 调用即 TypeError；若发生在 async 函数里又没被正确捕获，表现就是
 * 「点了没反应、没弹窗、没报错」。
 *
 * 语法必须保持 ES5：箭头函数/const/let 在目标内核上会直接解析失败。
 */
(function () {
  'use strict';

  // Chrome 86+。全站 28 处调用，房间发现、终端、工作区、导出中心都依赖它。
  if (typeof Element !== 'undefined' && !Element.prototype.replaceChildren) {
    Element.prototype.replaceChildren = function () {
      while (this.firstChild) this.removeChild(this.firstChild);
      for (var i = 0; i < arguments.length; i++) {
        var node = arguments[i];
        if (node === null || node === undefined) continue;
        this.appendChild(node.nodeType ? node : document.createTextNode(String(node)));
      }
    };
  }

  // Chrome 54+ 补齐 append / prepend（设置页分组用到）。
  if (typeof Element !== 'undefined' && !Element.prototype.append) {
    Element.prototype.append = function () {
      for (var i = 0; i < arguments.length; i++) {
        var node = arguments[i];
        if (node === null || node === undefined) continue;
        this.appendChild(node.nodeType ? node : document.createTextNode(String(node)));
      }
    };
  }
  if (typeof Element !== 'undefined' && !Element.prototype.prepend) {
    Element.prototype.prepend = function () {
      var first = this.firstChild;
      for (var i = 0; i < arguments.length; i++) {
        var node = arguments[i];
        if (node === null || node === undefined) continue;
        this.insertBefore(node.nodeType ? node : document.createTextNode(String(node)), first);
      }
    };
  }
  if (typeof Element !== 'undefined' && !Element.prototype.remove) {
    Element.prototype.remove = function () {
      if (this.parentNode) this.parentNode.removeChild(this);
    };
  }

  // Chrome 93+
  if (!Object.hasOwn) {
    Object.hasOwn = function (object, key) {
      return Object.prototype.hasOwnProperty.call(object, key);
    };
  }

  // Chrome 85+
  if (!String.prototype.replaceAll) {
    String.prototype.replaceAll = function (search, replacement) {
      if (search instanceof RegExp) {
        if (!search.global) throw new TypeError('replaceAll 的正则必须带 g 标志');
        return this.replace(search, replacement);
      }
      return this.split(String(search)).join(String(replacement));
    };
  }

  // Chrome 76+，旧内核可能缺失；不依赖它的业务代码也能跑，但补上更安全。
  if (!Promise.allSettled) {
    Promise.allSettled = function (list) {
      return Promise.all(
        Array.prototype.map.call(list, function (item) {
          return Promise.resolve(item).then(
            function (value) { return { status: 'fulfilled', value: value }; },
            function (reason) { return { status: 'rejected', reason: reason }; }
          );
        })
      );
    };
  }
})();
