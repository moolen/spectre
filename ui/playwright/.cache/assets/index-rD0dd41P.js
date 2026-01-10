const __vite__mapDeps=(i,m=__vite__mapDeps,d=(m.f||(m.f=["assets/App-CN82QFaE.js","assets/SettingsPage-DlyVciZE.js"])))=>i.map(i=>d[i]);
function _mergeNamespaces(n, m) {
  for (var i = 0; i < m.length; i++) {
    const e = m[i];
    if (typeof e !== 'string' && !Array.isArray(e)) { for (const k in e) {
      if (k !== 'default' && !(k in n)) {
        const d = Object.getOwnPropertyDescriptor(e, k);
        if (d) {
          Object.defineProperty(n, k, d.get ? d : {
            enumerable: true,
            get: () => e[k]
          });
        }
      }
    } }
  }
  return Object.freeze(Object.defineProperty(n, Symbol.toStringTag, { value: 'Module' }));
}

true              &&(function polyfill() {
  const relList = document.createElement("link").relList;
  if (relList && relList.supports && relList.supports("modulepreload")) {
    return;
  }
  for (const link of document.querySelectorAll('link[rel="modulepreload"]')) {
    processPreload(link);
  }
  new MutationObserver((mutations) => {
    for (const mutation of mutations) {
      if (mutation.type !== "childList") {
        continue;
      }
      for (const node of mutation.addedNodes) {
        if (node.tagName === "LINK" && node.rel === "modulepreload")
          processPreload(node);
      }
    }
  }).observe(document, { childList: true, subtree: true });
  function getFetchOpts(link) {
    const fetchOpts = {};
    if (link.integrity) fetchOpts.integrity = link.integrity;
    if (link.referrerPolicy) fetchOpts.referrerPolicy = link.referrerPolicy;
    if (link.crossOrigin === "use-credentials")
      fetchOpts.credentials = "include";
    else if (link.crossOrigin === "anonymous") fetchOpts.credentials = "omit";
    else fetchOpts.credentials = "same-origin";
    return fetchOpts;
  }
  function processPreload(link) {
    if (link.ep)
      return;
    link.ep = true;
    const fetchOpts = getFetchOpts(link);
    fetch(link.href, fetchOpts);
  }
}());

const index_html_htmlProxy_inlineCss_index_0 = '';

const scriptRel = 'modulepreload';const assetsURL = function(dep) { return "/"+dep };const seen = {};const __vitePreload = function preload(baseModule, deps, importerUrl) {
  let promise = Promise.resolve();
  if (true               && deps && deps.length > 0) {
    let allSettled2 = function(promises) {
      return Promise.all(
        promises.map(
          (p) => Promise.resolve(p).then(
            (value) => ({ status: "fulfilled", value }),
            (reason) => ({ status: "rejected", reason })
          )
        )
      );
    };
    const links = document.getElementsByTagName("link");
    const cspNonceMeta = document.querySelector(
      "meta[property=csp-nonce]"
    );
    const cspNonce = cspNonceMeta?.nonce || cspNonceMeta?.getAttribute("nonce");
    promise = allSettled2(
      deps.map((dep) => {
        dep = assetsURL(dep, importerUrl);
        if (dep in seen) return;
        seen[dep] = true;
        const isCss = dep.endsWith(".css");
        const cssSelector = isCss ? '[rel="stylesheet"]' : "";
        const isBaseRelative = !!importerUrl;
        if (isBaseRelative) {
          for (let i = links.length - 1; i >= 0; i--) {
            const link2 = links[i];
            if (link2.href === dep && (!isCss || link2.rel === "stylesheet")) {
              return;
            }
          }
        } else if (document.querySelector(`link[href="${dep}"]${cssSelector}`)) {
          return;
        }
        const link = document.createElement("link");
        link.rel = isCss ? "stylesheet" : scriptRel;
        if (!isCss) {
          link.as = "script";
        }
        link.crossOrigin = "";
        link.href = dep;
        if (cspNonce) {
          link.setAttribute("nonce", cspNonce);
        }
        document.head.appendChild(link);
        if (isCss) {
          return new Promise((res, rej) => {
            link.addEventListener("load", res);
            link.addEventListener(
              "error",
              () => rej(new Error(`Unable to preload CSS for ${dep}`))
            );
          });
        }
      })
    );
  }
  function handlePreloadError(err) {
    const e = new Event("vite:preloadError", {
      cancelable: true
    });
    e.payload = err;
    window.dispatchEvent(e);
    if (!e.defaultPrevented) {
      throw err;
    }
  }
  return promise.then((res) => {
    for (const item of res || []) {
      if (item.status !== "rejected") continue;
      handlePreloadError(item.reason);
    }
    return baseModule().catch(handlePreloadError);
  });
};

var commonjsGlobal = typeof globalThis !== 'undefined' ? globalThis : typeof window !== 'undefined' ? window : typeof global !== 'undefined' ? global : typeof self !== 'undefined' ? self : {};

function getDefaultExportFromCjs (x) {
	return x && x.__esModule && Object.prototype.hasOwnProperty.call(x, 'default') ? x['default'] : x;
}

function getDefaultExportFromNamespaceIfPresent (n) {
	return n && Object.prototype.hasOwnProperty.call(n, 'default') ? n['default'] : n;
}

function getDefaultExportFromNamespaceIfNotNamed (n) {
	return n && Object.prototype.hasOwnProperty.call(n, 'default') && Object.keys(n).length === 1 ? n['default'] : n;
}

function getAugmentedNamespace(n) {
  if (Object.prototype.hasOwnProperty.call(n, '__esModule')) return n;
  var f = n.default;
	if (typeof f == "function") {
		var a = function a () {
			if (this instanceof a) {
        return Reflect.construct(f, arguments, this.constructor);
			}
			return f.apply(this, arguments);
		};
		a.prototype = f.prototype;
  } else a = {};
  Object.defineProperty(a, '__esModule', {value: true});
	Object.keys(n).forEach(function (k) {
		var d = Object.getOwnPropertyDescriptor(n, k);
		Object.defineProperty(a, k, d.get ? d : {
			enumerable: true,
			get: function () {
				return n[k];
			}
		});
	});
	return a;
}

var jsxRuntime$2 = {exports: {}};

var reactJsxRuntime_production = {};

/**
 * @license React
 * react-jsx-runtime.production.js
 *
 * Copyright (c) Meta Platforms, Inc. and affiliates.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */

var hasRequiredReactJsxRuntime_production;

function requireReactJsxRuntime_production () {
	if (hasRequiredReactJsxRuntime_production) return reactJsxRuntime_production;
	hasRequiredReactJsxRuntime_production = 1;
	"use strict";
	var REACT_ELEMENT_TYPE = Symbol.for("react.transitional.element"),
	  REACT_FRAGMENT_TYPE = Symbol.for("react.fragment");
	function jsxProd(type, config, maybeKey) {
	  var key = null;
	  void 0 !== maybeKey && (key = "" + maybeKey);
	  void 0 !== config.key && (key = "" + config.key);
	  if ("key" in config) {
	    maybeKey = {};
	    for (var propName in config)
	      "key" !== propName && (maybeKey[propName] = config[propName]);
	  } else maybeKey = config;
	  config = maybeKey.ref;
	  return {
	    $$typeof: REACT_ELEMENT_TYPE,
	    type: type,
	    key: key,
	    ref: void 0 !== config ? config : null,
	    props: maybeKey
	  };
	}
	reactJsxRuntime_production.Fragment = REACT_FRAGMENT_TYPE;
	reactJsxRuntime_production.jsx = jsxProd;
	reactJsxRuntime_production.jsxs = jsxProd;
	return reactJsxRuntime_production;
}

var jsxRuntime$1 = jsxRuntime$2.exports;

var hasRequiredJsxRuntime;

function requireJsxRuntime () {
	if (hasRequiredJsxRuntime) return jsxRuntime$2.exports;
	hasRequiredJsxRuntime = 1;
	"use strict";
	if (true) {
	  jsxRuntime$2.exports = requireReactJsxRuntime_production();
	} else {
	  module.exports = require("./cjs/react-jsx-runtime.development.js");
	}
	return jsxRuntime$2.exports;
}

var jsxRuntimeExports = requireJsxRuntime();
const jsxRuntime = /*@__PURE__*/getDefaultExportFromCjs(jsxRuntimeExports);

/**
 * Copyright (c) Microsoft Corporation.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

const __pw_hooks_before_mount = [];
const __pw_hooks_after_mount = [];

window.__pw_hooks_before_mount = __pw_hooks_before_mount;
window.__pw_hooks_after_mount = __pw_hooks_after_mount;

const beforeMount = callback => {
  __pw_hooks_before_mount.push(callback);
};

const afterMount = callback => {
  __pw_hooks_after_mount.push(callback);
};

var react$1 = {exports: {}};

var react_production = {};

/**
 * @license React
 * react.production.js
 *
 * Copyright (c) Meta Platforms, Inc. and affiliates.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */

var hasRequiredReact_production;

function requireReact_production () {
	if (hasRequiredReact_production) return react_production;
	hasRequiredReact_production = 1;
	"use strict";
	var REACT_ELEMENT_TYPE = Symbol.for("react.transitional.element"),
	  REACT_PORTAL_TYPE = Symbol.for("react.portal"),
	  REACT_FRAGMENT_TYPE = Symbol.for("react.fragment"),
	  REACT_STRICT_MODE_TYPE = Symbol.for("react.strict_mode"),
	  REACT_PROFILER_TYPE = Symbol.for("react.profiler"),
	  REACT_CONSUMER_TYPE = Symbol.for("react.consumer"),
	  REACT_CONTEXT_TYPE = Symbol.for("react.context"),
	  REACT_FORWARD_REF_TYPE = Symbol.for("react.forward_ref"),
	  REACT_SUSPENSE_TYPE = Symbol.for("react.suspense"),
	  REACT_MEMO_TYPE = Symbol.for("react.memo"),
	  REACT_LAZY_TYPE = Symbol.for("react.lazy"),
	  REACT_ACTIVITY_TYPE = Symbol.for("react.activity"),
	  MAYBE_ITERATOR_SYMBOL = Symbol.iterator;
	function getIteratorFn(maybeIterable) {
	  if (null === maybeIterable || "object" !== typeof maybeIterable) return null;
	  maybeIterable =
	    (MAYBE_ITERATOR_SYMBOL && maybeIterable[MAYBE_ITERATOR_SYMBOL]) ||
	    maybeIterable["@@iterator"];
	  return "function" === typeof maybeIterable ? maybeIterable : null;
	}
	var ReactNoopUpdateQueue = {
	    isMounted: function () {
	      return !1;
	    },
	    enqueueForceUpdate: function () {},
	    enqueueReplaceState: function () {},
	    enqueueSetState: function () {}
	  },
	  assign = Object.assign,
	  emptyObject = {};
	function Component(props, context, updater) {
	  this.props = props;
	  this.context = context;
	  this.refs = emptyObject;
	  this.updater = updater || ReactNoopUpdateQueue;
	}
	Component.prototype.isReactComponent = {};
	Component.prototype.setState = function (partialState, callback) {
	  if (
	    "object" !== typeof partialState &&
	    "function" !== typeof partialState &&
	    null != partialState
	  )
	    throw Error(
	      "takes an object of state variables to update or a function which returns an object of state variables."
	    );
	  this.updater.enqueueSetState(this, partialState, callback, "setState");
	};
	Component.prototype.forceUpdate = function (callback) {
	  this.updater.enqueueForceUpdate(this, callback, "forceUpdate");
	};
	function ComponentDummy() {}
	ComponentDummy.prototype = Component.prototype;
	function PureComponent(props, context, updater) {
	  this.props = props;
	  this.context = context;
	  this.refs = emptyObject;
	  this.updater = updater || ReactNoopUpdateQueue;
	}
	var pureComponentPrototype = (PureComponent.prototype = new ComponentDummy());
	pureComponentPrototype.constructor = PureComponent;
	assign(pureComponentPrototype, Component.prototype);
	pureComponentPrototype.isPureReactComponent = !0;
	var isArrayImpl = Array.isArray;
	function noop() {}
	var ReactSharedInternals = { H: null, A: null, T: null, S: null },
	  hasOwnProperty = Object.prototype.hasOwnProperty;
	function ReactElement(type, key, props) {
	  var refProp = props.ref;
	  return {
	    $$typeof: REACT_ELEMENT_TYPE,
	    type: type,
	    key: key,
	    ref: void 0 !== refProp ? refProp : null,
	    props: props
	  };
	}
	function cloneAndReplaceKey(oldElement, newKey) {
	  return ReactElement(oldElement.type, newKey, oldElement.props);
	}
	function isValidElement(object) {
	  return (
	    "object" === typeof object &&
	    null !== object &&
	    object.$$typeof === REACT_ELEMENT_TYPE
	  );
	}
	function escape(key) {
	  var escaperLookup = { "=": "=0", ":": "=2" };
	  return (
	    "$" +
	    key.replace(/[=:]/g, function (match) {
	      return escaperLookup[match];
	    })
	  );
	}
	var userProvidedKeyEscapeRegex = /\/+/g;
	function getElementKey(element, index) {
	  return "object" === typeof element && null !== element && null != element.key
	    ? escape("" + element.key)
	    : index.toString(36);
	}
	function resolveThenable(thenable) {
	  switch (thenable.status) {
	    case "fulfilled":
	      return thenable.value;
	    case "rejected":
	      throw thenable.reason;
	    default:
	      switch (
	        ("string" === typeof thenable.status
	          ? thenable.then(noop, noop)
	          : ((thenable.status = "pending"),
	            thenable.then(
	              function (fulfilledValue) {
	                "pending" === thenable.status &&
	                  ((thenable.status = "fulfilled"),
	                  (thenable.value = fulfilledValue));
	              },
	              function (error) {
	                "pending" === thenable.status &&
	                  ((thenable.status = "rejected"), (thenable.reason = error));
	              }
	            )),
	        thenable.status)
	      ) {
	        case "fulfilled":
	          return thenable.value;
	        case "rejected":
	          throw thenable.reason;
	      }
	  }
	  throw thenable;
	}
	function mapIntoArray(children, array, escapedPrefix, nameSoFar, callback) {
	  var type = typeof children;
	  if ("undefined" === type || "boolean" === type) children = null;
	  var invokeCallback = !1;
	  if (null === children) invokeCallback = !0;
	  else
	    switch (type) {
	      case "bigint":
	      case "string":
	      case "number":
	        invokeCallback = !0;
	        break;
	      case "object":
	        switch (children.$$typeof) {
	          case REACT_ELEMENT_TYPE:
	          case REACT_PORTAL_TYPE:
	            invokeCallback = !0;
	            break;
	          case REACT_LAZY_TYPE:
	            return (
	              (invokeCallback = children._init),
	              mapIntoArray(
	                invokeCallback(children._payload),
	                array,
	                escapedPrefix,
	                nameSoFar,
	                callback
	              )
	            );
	        }
	    }
	  if (invokeCallback)
	    return (
	      (callback = callback(children)),
	      (invokeCallback =
	        "" === nameSoFar ? "." + getElementKey(children, 0) : nameSoFar),
	      isArrayImpl(callback)
	        ? ((escapedPrefix = ""),
	          null != invokeCallback &&
	            (escapedPrefix =
	              invokeCallback.replace(userProvidedKeyEscapeRegex, "$&/") + "/"),
	          mapIntoArray(callback, array, escapedPrefix, "", function (c) {
	            return c;
	          }))
	        : null != callback &&
	          (isValidElement(callback) &&
	            (callback = cloneAndReplaceKey(
	              callback,
	              escapedPrefix +
	                (null == callback.key ||
	                (children && children.key === callback.key)
	                  ? ""
	                  : ("" + callback.key).replace(
	                      userProvidedKeyEscapeRegex,
	                      "$&/"
	                    ) + "/") +
	                invokeCallback
	            )),
	          array.push(callback)),
	      1
	    );
	  invokeCallback = 0;
	  var nextNamePrefix = "" === nameSoFar ? "." : nameSoFar + ":";
	  if (isArrayImpl(children))
	    for (var i = 0; i < children.length; i++)
	      (nameSoFar = children[i]),
	        (type = nextNamePrefix + getElementKey(nameSoFar, i)),
	        (invokeCallback += mapIntoArray(
	          nameSoFar,
	          array,
	          escapedPrefix,
	          type,
	          callback
	        ));
	  else if (((i = getIteratorFn(children)), "function" === typeof i))
	    for (
	      children = i.call(children), i = 0;
	      !(nameSoFar = children.next()).done;

	    )
	      (nameSoFar = nameSoFar.value),
	        (type = nextNamePrefix + getElementKey(nameSoFar, i++)),
	        (invokeCallback += mapIntoArray(
	          nameSoFar,
	          array,
	          escapedPrefix,
	          type,
	          callback
	        ));
	  else if ("object" === type) {
	    if ("function" === typeof children.then)
	      return mapIntoArray(
	        resolveThenable(children),
	        array,
	        escapedPrefix,
	        nameSoFar,
	        callback
	      );
	    array = String(children);
	    throw Error(
	      "Objects are not valid as a React child (found: " +
	        ("[object Object]" === array
	          ? "object with keys {" + Object.keys(children).join(", ") + "}"
	          : array) +
	        "). If you meant to render a collection of children, use an array instead."
	    );
	  }
	  return invokeCallback;
	}
	function mapChildren(children, func, context) {
	  if (null == children) return children;
	  var result = [],
	    count = 0;
	  mapIntoArray(children, result, "", "", function (child) {
	    return func.call(context, child, count++);
	  });
	  return result;
	}
	function lazyInitializer(payload) {
	  if (-1 === payload._status) {
	    var ctor = payload._result;
	    ctor = ctor();
	    ctor.then(
	      function (moduleObject) {
	        if (0 === payload._status || -1 === payload._status)
	          (payload._status = 1), (payload._result = moduleObject);
	      },
	      function (error) {
	        if (0 === payload._status || -1 === payload._status)
	          (payload._status = 2), (payload._result = error);
	      }
	    );
	    -1 === payload._status && ((payload._status = 0), (payload._result = ctor));
	  }
	  if (1 === payload._status) return payload._result.default;
	  throw payload._result;
	}
	var reportGlobalError =
	    "function" === typeof reportError
	      ? reportError
	      : function (error) {
	          if (
	            "object" === typeof window &&
	            "function" === typeof window.ErrorEvent
	          ) {
	            var event = new window.ErrorEvent("error", {
	              bubbles: !0,
	              cancelable: !0,
	              message:
	                "object" === typeof error &&
	                null !== error &&
	                "string" === typeof error.message
	                  ? String(error.message)
	                  : String(error),
	              error: error
	            });
	            if (!window.dispatchEvent(event)) return;
	          } else if (
	            "object" === typeof process &&
	            "function" === typeof process.emit
	          ) {
	            process.emit("uncaughtException", error);
	            return;
	          }
	          console.error(error);
	        },
	  Children = {
	    map: mapChildren,
	    forEach: function (children, forEachFunc, forEachContext) {
	      mapChildren(
	        children,
	        function () {
	          forEachFunc.apply(this, arguments);
	        },
	        forEachContext
	      );
	    },
	    count: function (children) {
	      var n = 0;
	      mapChildren(children, function () {
	        n++;
	      });
	      return n;
	    },
	    toArray: function (children) {
	      return (
	        mapChildren(children, function (child) {
	          return child;
	        }) || []
	      );
	    },
	    only: function (children) {
	      if (!isValidElement(children))
	        throw Error(
	          "React.Children.only expected to receive a single React element child."
	        );
	      return children;
	    }
	  };
	react_production.Activity = REACT_ACTIVITY_TYPE;
	react_production.Children = Children;
	react_production.Component = Component;
	react_production.Fragment = REACT_FRAGMENT_TYPE;
	react_production.Profiler = REACT_PROFILER_TYPE;
	react_production.PureComponent = PureComponent;
	react_production.StrictMode = REACT_STRICT_MODE_TYPE;
	react_production.Suspense = REACT_SUSPENSE_TYPE;
	react_production.__CLIENT_INTERNALS_DO_NOT_USE_OR_WARN_USERS_THEY_CANNOT_UPGRADE =
	  ReactSharedInternals;
	react_production.__COMPILER_RUNTIME = {
	  __proto__: null,
	  c: function (size) {
	    return ReactSharedInternals.H.useMemoCache(size);
	  }
	};
	react_production.cache = function (fn) {
	  return function () {
	    return fn.apply(null, arguments);
	  };
	};
	react_production.cacheSignal = function () {
	  return null;
	};
	react_production.cloneElement = function (element, config, children) {
	  if (null === element || void 0 === element)
	    throw Error(
	      "The argument must be a React element, but you passed " + element + "."
	    );
	  var props = assign({}, element.props),
	    key = element.key;
	  if (null != config)
	    for (propName in (void 0 !== config.key && (key = "" + config.key), config))
	      !hasOwnProperty.call(config, propName) ||
	        "key" === propName ||
	        "__self" === propName ||
	        "__source" === propName ||
	        ("ref" === propName && void 0 === config.ref) ||
	        (props[propName] = config[propName]);
	  var propName = arguments.length - 2;
	  if (1 === propName) props.children = children;
	  else if (1 < propName) {
	    for (var childArray = Array(propName), i = 0; i < propName; i++)
	      childArray[i] = arguments[i + 2];
	    props.children = childArray;
	  }
	  return ReactElement(element.type, key, props);
	};
	react_production.createContext = function (defaultValue) {
	  defaultValue = {
	    $$typeof: REACT_CONTEXT_TYPE,
	    _currentValue: defaultValue,
	    _currentValue2: defaultValue,
	    _threadCount: 0,
	    Provider: null,
	    Consumer: null
	  };
	  defaultValue.Provider = defaultValue;
	  defaultValue.Consumer = {
	    $$typeof: REACT_CONSUMER_TYPE,
	    _context: defaultValue
	  };
	  return defaultValue;
	};
	react_production.createElement = function (type, config, children) {
	  var propName,
	    props = {},
	    key = null;
	  if (null != config)
	    for (propName in (void 0 !== config.key && (key = "" + config.key), config))
	      hasOwnProperty.call(config, propName) &&
	        "key" !== propName &&
	        "__self" !== propName &&
	        "__source" !== propName &&
	        (props[propName] = config[propName]);
	  var childrenLength = arguments.length - 2;
	  if (1 === childrenLength) props.children = children;
	  else if (1 < childrenLength) {
	    for (var childArray = Array(childrenLength), i = 0; i < childrenLength; i++)
	      childArray[i] = arguments[i + 2];
	    props.children = childArray;
	  }
	  if (type && type.defaultProps)
	    for (propName in ((childrenLength = type.defaultProps), childrenLength))
	      void 0 === props[propName] &&
	        (props[propName] = childrenLength[propName]);
	  return ReactElement(type, key, props);
	};
	react_production.createRef = function () {
	  return { current: null };
	};
	react_production.forwardRef = function (render) {
	  return { $$typeof: REACT_FORWARD_REF_TYPE, render: render };
	};
	react_production.isValidElement = isValidElement;
	react_production.lazy = function (ctor) {
	  return {
	    $$typeof: REACT_LAZY_TYPE,
	    _payload: { _status: -1, _result: ctor },
	    _init: lazyInitializer
	  };
	};
	react_production.memo = function (type, compare) {
	  return {
	    $$typeof: REACT_MEMO_TYPE,
	    type: type,
	    compare: void 0 === compare ? null : compare
	  };
	};
	react_production.startTransition = function (scope) {
	  var prevTransition = ReactSharedInternals.T,
	    currentTransition = {};
	  ReactSharedInternals.T = currentTransition;
	  try {
	    var returnValue = scope(),
	      onStartTransitionFinish = ReactSharedInternals.S;
	    null !== onStartTransitionFinish &&
	      onStartTransitionFinish(currentTransition, returnValue);
	    "object" === typeof returnValue &&
	      null !== returnValue &&
	      "function" === typeof returnValue.then &&
	      returnValue.then(noop, reportGlobalError);
	  } catch (error) {
	    reportGlobalError(error);
	  } finally {
	    null !== prevTransition &&
	      null !== currentTransition.types &&
	      (prevTransition.types = currentTransition.types),
	      (ReactSharedInternals.T = prevTransition);
	  }
	};
	react_production.unstable_useCacheRefresh = function () {
	  return ReactSharedInternals.H.useCacheRefresh();
	};
	react_production.use = function (usable) {
	  return ReactSharedInternals.H.use(usable);
	};
	react_production.useActionState = function (action, initialState, permalink) {
	  return ReactSharedInternals.H.useActionState(action, initialState, permalink);
	};
	react_production.useCallback = function (callback, deps) {
	  return ReactSharedInternals.H.useCallback(callback, deps);
	};
	react_production.useContext = function (Context) {
	  return ReactSharedInternals.H.useContext(Context);
	};
	react_production.useDebugValue = function () {};
	react_production.useDeferredValue = function (value, initialValue) {
	  return ReactSharedInternals.H.useDeferredValue(value, initialValue);
	};
	react_production.useEffect = function (create, deps) {
	  return ReactSharedInternals.H.useEffect(create, deps);
	};
	react_production.useEffectEvent = function (callback) {
	  return ReactSharedInternals.H.useEffectEvent(callback);
	};
	react_production.useId = function () {
	  return ReactSharedInternals.H.useId();
	};
	react_production.useImperativeHandle = function (ref, create, deps) {
	  return ReactSharedInternals.H.useImperativeHandle(ref, create, deps);
	};
	react_production.useInsertionEffect = function (create, deps) {
	  return ReactSharedInternals.H.useInsertionEffect(create, deps);
	};
	react_production.useLayoutEffect = function (create, deps) {
	  return ReactSharedInternals.H.useLayoutEffect(create, deps);
	};
	react_production.useMemo = function (create, deps) {
	  return ReactSharedInternals.H.useMemo(create, deps);
	};
	react_production.useOptimistic = function (passthrough, reducer) {
	  return ReactSharedInternals.H.useOptimistic(passthrough, reducer);
	};
	react_production.useReducer = function (reducer, initialArg, init) {
	  return ReactSharedInternals.H.useReducer(reducer, initialArg, init);
	};
	react_production.useRef = function (initialValue) {
	  return ReactSharedInternals.H.useRef(initialValue);
	};
	react_production.useState = function (initialState) {
	  return ReactSharedInternals.H.useState(initialState);
	};
	react_production.useSyncExternalStore = function (
	  subscribe,
	  getSnapshot,
	  getServerSnapshot
	) {
	  return ReactSharedInternals.H.useSyncExternalStore(
	    subscribe,
	    getSnapshot,
	    getServerSnapshot
	  );
	};
	react_production.useTransition = function () {
	  return ReactSharedInternals.H.useTransition();
	};
	react_production.version = "19.2.3";
	return react_production;
}

var react = react$1.exports;

var hasRequiredReact;

function requireReact () {
	if (hasRequiredReact) return react$1.exports;
	hasRequiredReact = 1;
	"use strict";
	if (true) {
	  react$1.exports = requireReact_production();
	} else {
	  module.exports = require("./cjs/react.development.js");
	}
	return react$1.exports;
}

var reactExports = requireReact();
const React = /*@__PURE__*/getDefaultExportFromCjs(reactExports);

const React$1 = /*#__PURE__*/_mergeNamespaces({
  __proto__: null,
  default: React
}, [reactExports]);

var reactDom$1 = {exports: {}};

var reactDom_production = {};

/**
 * @license React
 * react-dom.production.js
 *
 * Copyright (c) Meta Platforms, Inc. and affiliates.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */

var hasRequiredReactDom_production;

function requireReactDom_production () {
	if (hasRequiredReactDom_production) return reactDom_production;
	hasRequiredReactDom_production = 1;
	"use strict";
	var React = requireReact();
	function formatProdErrorMessage(code) {
	  var url = "https://react.dev/errors/" + code;
	  if (1 < arguments.length) {
	    url += "?args[]=" + encodeURIComponent(arguments[1]);
	    for (var i = 2; i < arguments.length; i++)
	      url += "&args[]=" + encodeURIComponent(arguments[i]);
	  }
	  return (
	    "Minified React error #" +
	    code +
	    "; visit " +
	    url +
	    " for the full message or use the non-minified dev environment for full errors and additional helpful warnings."
	  );
	}
	function noop() {}
	var Internals = {
	    d: {
	      f: noop,
	      r: function () {
	        throw Error(formatProdErrorMessage(522));
	      },
	      D: noop,
	      C: noop,
	      L: noop,
	      m: noop,
	      X: noop,
	      S: noop,
	      M: noop
	    },
	    p: 0,
	    findDOMNode: null
	  },
	  REACT_PORTAL_TYPE = Symbol.for("react.portal");
	function createPortal$1(children, containerInfo, implementation) {
	  var key =
	    3 < arguments.length && void 0 !== arguments[3] ? arguments[3] : null;
	  return {
	    $$typeof: REACT_PORTAL_TYPE,
	    key: null == key ? null : "" + key,
	    children: children,
	    containerInfo: containerInfo,
	    implementation: implementation
	  };
	}
	var ReactSharedInternals =
	  React.__CLIENT_INTERNALS_DO_NOT_USE_OR_WARN_USERS_THEY_CANNOT_UPGRADE;
	function getCrossOriginStringAs(as, input) {
	  if ("font" === as) return "";
	  if ("string" === typeof input)
	    return "use-credentials" === input ? input : "";
	}
	reactDom_production.__DOM_INTERNALS_DO_NOT_USE_OR_WARN_USERS_THEY_CANNOT_UPGRADE =
	  Internals;
	reactDom_production.createPortal = function (children, container) {
	  var key =
	    2 < arguments.length && void 0 !== arguments[2] ? arguments[2] : null;
	  if (
	    !container ||
	    (1 !== container.nodeType &&
	      9 !== container.nodeType &&
	      11 !== container.nodeType)
	  )
	    throw Error(formatProdErrorMessage(299));
	  return createPortal$1(children, container, null, key);
	};
	reactDom_production.flushSync = function (fn) {
	  var previousTransition = ReactSharedInternals.T,
	    previousUpdatePriority = Internals.p;
	  try {
	    if (((ReactSharedInternals.T = null), (Internals.p = 2), fn)) return fn();
	  } finally {
	    (ReactSharedInternals.T = previousTransition),
	      (Internals.p = previousUpdatePriority),
	      Internals.d.f();
	  }
	};
	reactDom_production.preconnect = function (href, options) {
	  "string" === typeof href &&
	    (options
	      ? ((options = options.crossOrigin),
	        (options =
	          "string" === typeof options
	            ? "use-credentials" === options
	              ? options
	              : ""
	            : void 0))
	      : (options = null),
	    Internals.d.C(href, options));
	};
	reactDom_production.prefetchDNS = function (href) {
	  "string" === typeof href && Internals.d.D(href);
	};
	reactDom_production.preinit = function (href, options) {
	  if ("string" === typeof href && options && "string" === typeof options.as) {
	    var as = options.as,
	      crossOrigin = getCrossOriginStringAs(as, options.crossOrigin),
	      integrity =
	        "string" === typeof options.integrity ? options.integrity : void 0,
	      fetchPriority =
	        "string" === typeof options.fetchPriority
	          ? options.fetchPriority
	          : void 0;
	    "style" === as
	      ? Internals.d.S(
	          href,
	          "string" === typeof options.precedence ? options.precedence : void 0,
	          {
	            crossOrigin: crossOrigin,
	            integrity: integrity,
	            fetchPriority: fetchPriority
	          }
	        )
	      : "script" === as &&
	        Internals.d.X(href, {
	          crossOrigin: crossOrigin,
	          integrity: integrity,
	          fetchPriority: fetchPriority,
	          nonce: "string" === typeof options.nonce ? options.nonce : void 0
	        });
	  }
	};
	reactDom_production.preinitModule = function (href, options) {
	  if ("string" === typeof href)
	    if ("object" === typeof options && null !== options) {
	      if (null == options.as || "script" === options.as) {
	        var crossOrigin = getCrossOriginStringAs(
	          options.as,
	          options.crossOrigin
	        );
	        Internals.d.M(href, {
	          crossOrigin: crossOrigin,
	          integrity:
	            "string" === typeof options.integrity ? options.integrity : void 0,
	          nonce: "string" === typeof options.nonce ? options.nonce : void 0
	        });
	      }
	    } else null == options && Internals.d.M(href);
	};
	reactDom_production.preload = function (href, options) {
	  if (
	    "string" === typeof href &&
	    "object" === typeof options &&
	    null !== options &&
	    "string" === typeof options.as
	  ) {
	    var as = options.as,
	      crossOrigin = getCrossOriginStringAs(as, options.crossOrigin);
	    Internals.d.L(href, as, {
	      crossOrigin: crossOrigin,
	      integrity:
	        "string" === typeof options.integrity ? options.integrity : void 0,
	      nonce: "string" === typeof options.nonce ? options.nonce : void 0,
	      type: "string" === typeof options.type ? options.type : void 0,
	      fetchPriority:
	        "string" === typeof options.fetchPriority
	          ? options.fetchPriority
	          : void 0,
	      referrerPolicy:
	        "string" === typeof options.referrerPolicy
	          ? options.referrerPolicy
	          : void 0,
	      imageSrcSet:
	        "string" === typeof options.imageSrcSet ? options.imageSrcSet : void 0,
	      imageSizes:
	        "string" === typeof options.imageSizes ? options.imageSizes : void 0,
	      media: "string" === typeof options.media ? options.media : void 0
	    });
	  }
	};
	reactDom_production.preloadModule = function (href, options) {
	  if ("string" === typeof href)
	    if (options) {
	      var crossOrigin = getCrossOriginStringAs(options.as, options.crossOrigin);
	      Internals.d.m(href, {
	        as:
	          "string" === typeof options.as && "script" !== options.as
	            ? options.as
	            : void 0,
	        crossOrigin: crossOrigin,
	        integrity:
	          "string" === typeof options.integrity ? options.integrity : void 0
	      });
	    } else Internals.d.m(href);
	};
	reactDom_production.requestFormReset = function (form) {
	  Internals.d.r(form);
	};
	reactDom_production.unstable_batchedUpdates = function (fn, a) {
	  return fn(a);
	};
	reactDom_production.useFormState = function (action, initialState, permalink) {
	  return ReactSharedInternals.H.useFormState(action, initialState, permalink);
	};
	reactDom_production.useFormStatus = function () {
	  return ReactSharedInternals.H.useHostTransitionStatus();
	};
	reactDom_production.version = "19.2.3";
	return reactDom_production;
}

var reactDom = reactDom$1.exports;

var hasRequiredReactDom;

function requireReactDom () {
	if (hasRequiredReactDom) return reactDom$1.exports;
	hasRequiredReactDom = 1;
	"use strict";
	function checkDCE() {
	  if (typeof __REACT_DEVTOOLS_GLOBAL_HOOK__ === "undefined" || typeof __REACT_DEVTOOLS_GLOBAL_HOOK__.checkDCE !== "function") {
	    return;
	  }
	  if (false) {
	    throw new Error("^_^");
	  }
	  try {
	    __REACT_DEVTOOLS_GLOBAL_HOOK__.checkDCE(checkDCE);
	  } catch (err) {
	    console.error(err);
	  }
	}
	if (true) {
	  checkDCE();
	  reactDom$1.exports = requireReactDom_production();
	} else {
	  module.exports = require("./cjs/react-dom.development.js");
	}
	return reactDom$1.exports;
}

var reactDomExports = requireReactDom();
const ReactDOM = /*@__PURE__*/getDefaultExportFromCjs(reactDomExports);

const ReactDOM$1 = /*#__PURE__*/_mergeNamespaces({
  __proto__: null,
  default: ReactDOM
}, [reactDomExports]);

/**
 * @remix-run/router v1.23.1
 *
 * Copyright (c) Remix Software Inc.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE.md file in the root directory of this source tree.
 *
 * @license MIT
 */
function _extends$2() {
  _extends$2 = Object.assign ? Object.assign.bind() : function (target) {
    for (var i = 1; i < arguments.length; i++) {
      var source = arguments[i];
      for (var key in source) {
        if (Object.prototype.hasOwnProperty.call(source, key)) {
          target[key] = source[key];
        }
      }
    }
    return target;
  };
  return _extends$2.apply(this, arguments);
}

////////////////////////////////////////////////////////////////////////////////
//#region Types and Constants
////////////////////////////////////////////////////////////////////////////////
/**
 * Actions represent the type of change to a location value.
 */
var Action;
(function (Action) {
  /**
   * A POP indicates a change to an arbitrary index in the history stack, such
   * as a back or forward navigation. It does not describe the direction of the
   * navigation, only that the current index changed.
   *
   * Note: This is the default action for newly created history objects.
   */
  Action["Pop"] = "POP";
  /**
   * A PUSH indicates a new entry being added to the history stack, such as when
   * a link is clicked and a new page loads. When this happens, all subsequent
   * entries in the stack are lost.
   */
  Action["Push"] = "PUSH";
  /**
   * A REPLACE indicates the entry at the current index in the history stack
   * being replaced by a new one.
   */
  Action["Replace"] = "REPLACE";
})(Action || (Action = {}));
const PopStateEventType = "popstate";
/**
 * Memory history stores the current location in memory. It is designed for use
 * in stateful non-browser environments like tests and React Native.
 */
function createMemoryHistory(options) {
  if (options === void 0) {
    options = {};
  }
  let {
    initialEntries = ["/"],
    initialIndex,
    v5Compat = false
  } = options;
  let entries; // Declare so we can access from createMemoryLocation
  entries = initialEntries.map((entry, index) => createMemoryLocation(entry, typeof entry === "string" ? null : entry.state, index === 0 ? "default" : undefined));
  let index = clampIndex(initialIndex == null ? entries.length - 1 : initialIndex);
  let action = Action.Pop;
  let listener = null;
  function clampIndex(n) {
    return Math.min(Math.max(n, 0), entries.length - 1);
  }
  function getCurrentLocation() {
    return entries[index];
  }
  function createMemoryLocation(to, state, key) {
    if (state === void 0) {
      state = null;
    }
    let location = createLocation(entries ? getCurrentLocation().pathname : "/", to, state, key);
    warning(location.pathname.charAt(0) === "/", "relative pathnames are not supported in memory history: " + JSON.stringify(to));
    return location;
  }
  function createHref(to) {
    return typeof to === "string" ? to : createPath(to);
  }
  let history = {
    get index() {
      return index;
    },
    get action() {
      return action;
    },
    get location() {
      return getCurrentLocation();
    },
    createHref,
    createURL(to) {
      return new URL(createHref(to), "http://localhost");
    },
    encodeLocation(to) {
      let path = typeof to === "string" ? parsePath(to) : to;
      return {
        pathname: path.pathname || "",
        search: path.search || "",
        hash: path.hash || ""
      };
    },
    push(to, state) {
      action = Action.Push;
      let nextLocation = createMemoryLocation(to, state);
      index += 1;
      entries.splice(index, entries.length, nextLocation);
      if (v5Compat && listener) {
        listener({
          action,
          location: nextLocation,
          delta: 1
        });
      }
    },
    replace(to, state) {
      action = Action.Replace;
      let nextLocation = createMemoryLocation(to, state);
      entries[index] = nextLocation;
      if (v5Compat && listener) {
        listener({
          action,
          location: nextLocation,
          delta: 0
        });
      }
    },
    go(delta) {
      action = Action.Pop;
      let nextIndex = clampIndex(index + delta);
      let nextLocation = entries[nextIndex];
      index = nextIndex;
      if (listener) {
        listener({
          action,
          location: nextLocation,
          delta
        });
      }
    },
    listen(fn) {
      listener = fn;
      return () => {
        listener = null;
      };
    }
  };
  return history;
}
/**
 * Browser history stores the location in regular URLs. This is the standard for
 * most web apps, but it requires some configuration on the server to ensure you
 * serve the same app at multiple URLs.
 *
 * @see https://github.com/remix-run/history/tree/main/docs/api-reference.md#createbrowserhistory
 */
function createBrowserHistory(options) {
  if (options === void 0) {
    options = {};
  }
  function createBrowserLocation(window, globalHistory) {
    let {
      pathname,
      search,
      hash
    } = window.location;
    return createLocation("", {
      pathname,
      search,
      hash
    },
    // state defaults to `null` because `window.history.state` does
    globalHistory.state && globalHistory.state.usr || null, globalHistory.state && globalHistory.state.key || "default");
  }
  function createBrowserHref(window, to) {
    return typeof to === "string" ? to : createPath(to);
  }
  return getUrlBasedHistory(createBrowserLocation, createBrowserHref, null, options);
}
/**
 * Hash history stores the location in window.location.hash. This makes it ideal
 * for situations where you don't want to send the location to the server for
 * some reason, either because you do cannot configure it or the URL space is
 * reserved for something else.
 *
 * @see https://github.com/remix-run/history/tree/main/docs/api-reference.md#createhashhistory
 */
function createHashHistory(options) {
  if (options === void 0) {
    options = {};
  }
  function createHashLocation(window, globalHistory) {
    let {
      pathname = "/",
      search = "",
      hash = ""
    } = parsePath(window.location.hash.substr(1));
    // Hash URL should always have a leading / just like window.location.pathname
    // does, so if an app ends up at a route like /#something then we add a
    // leading slash so all of our path-matching behaves the same as if it would
    // in a browser router.  This is particularly important when there exists a
    // root splat route (<Route path="*">) since that matches internally against
    // "/*" and we'd expect /#something to 404 in a hash router app.
    if (!pathname.startsWith("/") && !pathname.startsWith(".")) {
      pathname = "/" + pathname;
    }
    return createLocation("", {
      pathname,
      search,
      hash
    },
    // state defaults to `null` because `window.history.state` does
    globalHistory.state && globalHistory.state.usr || null, globalHistory.state && globalHistory.state.key || "default");
  }
  function createHashHref(window, to) {
    let base = window.document.querySelector("base");
    let href = "";
    if (base && base.getAttribute("href")) {
      let url = window.location.href;
      let hashIndex = url.indexOf("#");
      href = hashIndex === -1 ? url : url.slice(0, hashIndex);
    }
    return href + "#" + (typeof to === "string" ? to : createPath(to));
  }
  function validateHashLocation(location, to) {
    warning(location.pathname.charAt(0) === "/", "relative pathnames are not supported in hash history.push(" + JSON.stringify(to) + ")");
  }
  return getUrlBasedHistory(createHashLocation, createHashHref, validateHashLocation, options);
}
function invariant(value, message) {
  if (value === false || value === null || typeof value === "undefined") {
    throw new Error(message);
  }
}
function warning(cond, message) {
  if (!cond) {
    // eslint-disable-next-line no-console
    if (typeof console !== "undefined") console.warn(message);
    try {
      // Welcome to debugging history!
      //
      // This error is thrown as a convenience, so you can more easily
      // find the source for a warning that appears in the console by
      // enabling "pause on exceptions" in your JavaScript debugger.
      throw new Error(message);
      // eslint-disable-next-line no-empty
    } catch (e) {}
  }
}
function createKey() {
  return Math.random().toString(36).substr(2, 8);
}
/**
 * For browser-based histories, we combine the state and key into an object
 */
function getHistoryState(location, index) {
  return {
    usr: location.state,
    key: location.key,
    idx: index
  };
}
/**
 * Creates a Location object with a unique key from the given Path
 */
function createLocation(current, to, state, key) {
  if (state === void 0) {
    state = null;
  }
  let location = _extends$2({
    pathname: typeof current === "string" ? current : current.pathname,
    search: "",
    hash: ""
  }, typeof to === "string" ? parsePath(to) : to, {
    state,
    // TODO: This could be cleaned up.  push/replace should probably just take
    // full Locations now and avoid the need to run through this flow at all
    // But that's a pretty big refactor to the current test suite so going to
    // keep as is for the time being and just let any incoming keys take precedence
    key: to && to.key || key || createKey()
  });
  return location;
}
/**
 * Creates a string URL path from the given pathname, search, and hash components.
 */
function createPath(_ref) {
  let {
    pathname = "/",
    search = "",
    hash = ""
  } = _ref;
  if (search && search !== "?") pathname += search.charAt(0) === "?" ? search : "?" + search;
  if (hash && hash !== "#") pathname += hash.charAt(0) === "#" ? hash : "#" + hash;
  return pathname;
}
/**
 * Parses a string URL path into its separate pathname, search, and hash components.
 */
function parsePath(path) {
  let parsedPath = {};
  if (path) {
    let hashIndex = path.indexOf("#");
    if (hashIndex >= 0) {
      parsedPath.hash = path.substr(hashIndex);
      path = path.substr(0, hashIndex);
    }
    let searchIndex = path.indexOf("?");
    if (searchIndex >= 0) {
      parsedPath.search = path.substr(searchIndex);
      path = path.substr(0, searchIndex);
    }
    if (path) {
      parsedPath.pathname = path;
    }
  }
  return parsedPath;
}
function getUrlBasedHistory(getLocation, createHref, validateLocation, options) {
  if (options === void 0) {
    options = {};
  }
  let {
    window = document.defaultView,
    v5Compat = false
  } = options;
  let globalHistory = window.history;
  let action = Action.Pop;
  let listener = null;
  let index = getIndex();
  // Index should only be null when we initialize. If not, it's because the
  // user called history.pushState or history.replaceState directly, in which
  // case we should log a warning as it will result in bugs.
  if (index == null) {
    index = 0;
    globalHistory.replaceState(_extends$2({}, globalHistory.state, {
      idx: index
    }), "");
  }
  function getIndex() {
    let state = globalHistory.state || {
      idx: null
    };
    return state.idx;
  }
  function handlePop() {
    action = Action.Pop;
    let nextIndex = getIndex();
    let delta = nextIndex == null ? null : nextIndex - index;
    index = nextIndex;
    if (listener) {
      listener({
        action,
        location: history.location,
        delta
      });
    }
  }
  function push(to, state) {
    action = Action.Push;
    let location = createLocation(history.location, to, state);
    if (validateLocation) validateLocation(location, to);
    index = getIndex() + 1;
    let historyState = getHistoryState(location, index);
    let url = history.createHref(location);
    // try...catch because iOS limits us to 100 pushState calls :/
    try {
      globalHistory.pushState(historyState, "", url);
    } catch (error) {
      // If the exception is because `state` can't be serialized, let that throw
      // outwards just like a replace call would so the dev knows the cause
      // https://html.spec.whatwg.org/multipage/nav-history-apis.html#shared-history-push/replace-state-steps
      // https://html.spec.whatwg.org/multipage/structured-data.html#structuredserializeinternal
      if (error instanceof DOMException && error.name === "DataCloneError") {
        throw error;
      }
      // They are going to lose state here, but there is no real
      // way to warn them about it since the page will refresh...
      window.location.assign(url);
    }
    if (v5Compat && listener) {
      listener({
        action,
        location: history.location,
        delta: 1
      });
    }
  }
  function replace(to, state) {
    action = Action.Replace;
    let location = createLocation(history.location, to, state);
    if (validateLocation) validateLocation(location, to);
    index = getIndex();
    let historyState = getHistoryState(location, index);
    let url = history.createHref(location);
    globalHistory.replaceState(historyState, "", url);
    if (v5Compat && listener) {
      listener({
        action,
        location: history.location,
        delta: 0
      });
    }
  }
  function createURL(to) {
    // window.location.origin is "null" (the literal string value) in Firefox
    // under certain conditions, notably when serving from a local HTML file
    // See https://bugzilla.mozilla.org/show_bug.cgi?id=878297
    let base = window.location.origin !== "null" ? window.location.origin : window.location.href;
    let href = typeof to === "string" ? to : createPath(to);
    // Treating this as a full URL will strip any trailing spaces so we need to
    // pre-encode them since they might be part of a matching splat param from
    // an ancestor route
    href = href.replace(/ $/, "%20");
    invariant(base, "No window.location.(origin|href) available to create URL for href: " + href);
    return new URL(href, base);
  }
  let history = {
    get action() {
      return action;
    },
    get location() {
      return getLocation(window, globalHistory);
    },
    listen(fn) {
      if (listener) {
        throw new Error("A history only accepts one active listener");
      }
      window.addEventListener(PopStateEventType, handlePop);
      listener = fn;
      return () => {
        window.removeEventListener(PopStateEventType, handlePop);
        listener = null;
      };
    },
    createHref(to) {
      return createHref(window, to);
    },
    createURL,
    encodeLocation(to) {
      // Encode a Location the same way window.location would
      let url = createURL(to);
      return {
        pathname: url.pathname,
        search: url.search,
        hash: url.hash
      };
    },
    push,
    replace,
    go(n) {
      return globalHistory.go(n);
    }
  };
  return history;
}
//#endregion

var ResultType;
(function (ResultType) {
  ResultType["data"] = "data";
  ResultType["deferred"] = "deferred";
  ResultType["redirect"] = "redirect";
  ResultType["error"] = "error";
})(ResultType || (ResultType = {}));
const immutableRouteKeys = new Set(["lazy", "caseSensitive", "path", "id", "index", "children"]);
function isIndexRoute(route) {
  return route.index === true;
}
// Walk the route tree generating unique IDs where necessary, so we are working
// solely with AgnosticDataRouteObject's within the Router
function convertRoutesToDataRoutes(routes, mapRouteProperties, parentPath, manifest) {
  if (parentPath === void 0) {
    parentPath = [];
  }
  if (manifest === void 0) {
    manifest = {};
  }
  return routes.map((route, index) => {
    let treePath = [...parentPath, String(index)];
    let id = typeof route.id === "string" ? route.id : treePath.join("-");
    invariant(route.index !== true || !route.children, "Cannot specify children on an index route");
    invariant(!manifest[id], "Found a route id collision on id \"" + id + "\".  Route " + "id's must be globally unique within Data Router usages");
    if (isIndexRoute(route)) {
      let indexRoute = _extends$2({}, route, mapRouteProperties(route), {
        id
      });
      manifest[id] = indexRoute;
      return indexRoute;
    } else {
      let pathOrLayoutRoute = _extends$2({}, route, mapRouteProperties(route), {
        id,
        children: undefined
      });
      manifest[id] = pathOrLayoutRoute;
      if (route.children) {
        pathOrLayoutRoute.children = convertRoutesToDataRoutes(route.children, mapRouteProperties, treePath, manifest);
      }
      return pathOrLayoutRoute;
    }
  });
}
/**
 * Matches the given routes to a location and returns the match data.
 *
 * @see https://reactrouter.com/v6/utils/match-routes
 */
function matchRoutes(routes, locationArg, basename) {
  if (basename === void 0) {
    basename = "/";
  }
  return matchRoutesImpl(routes, locationArg, basename, false);
}
function matchRoutesImpl(routes, locationArg, basename, allowPartial) {
  let location = typeof locationArg === "string" ? parsePath(locationArg) : locationArg;
  let pathname = stripBasename(location.pathname || "/", basename);
  if (pathname == null) {
    return null;
  }
  let branches = flattenRoutes(routes);
  rankRouteBranches(branches);
  let matches = null;
  for (let i = 0; matches == null && i < branches.length; ++i) {
    // Incoming pathnames are generally encoded from either window.location
    // or from router.navigate, but we want to match against the unencoded
    // paths in the route definitions.  Memory router locations won't be
    // encoded here but there also shouldn't be anything to decode so this
    // should be a safe operation.  This avoids needing matchRoutes to be
    // history-aware.
    let decoded = decodePath(pathname);
    matches = matchRouteBranch(branches[i], decoded, allowPartial);
  }
  return matches;
}
function convertRouteMatchToUiMatch(match, loaderData) {
  let {
    route,
    pathname,
    params
  } = match;
  return {
    id: route.id,
    pathname,
    params,
    data: loaderData[route.id],
    handle: route.handle
  };
}
function flattenRoutes(routes, branches, parentsMeta, parentPath) {
  if (branches === void 0) {
    branches = [];
  }
  if (parentsMeta === void 0) {
    parentsMeta = [];
  }
  if (parentPath === void 0) {
    parentPath = "";
  }
  let flattenRoute = (route, index, relativePath) => {
    let meta = {
      relativePath: relativePath === undefined ? route.path || "" : relativePath,
      caseSensitive: route.caseSensitive === true,
      childrenIndex: index,
      route
    };
    if (meta.relativePath.startsWith("/")) {
      invariant(meta.relativePath.startsWith(parentPath), "Absolute route path \"" + meta.relativePath + "\" nested under path " + ("\"" + parentPath + "\" is not valid. An absolute child route path ") + "must start with the combined path of all its parent routes.");
      meta.relativePath = meta.relativePath.slice(parentPath.length);
    }
    let path = joinPaths([parentPath, meta.relativePath]);
    let routesMeta = parentsMeta.concat(meta);
    // Add the children before adding this route to the array, so we traverse the
    // route tree depth-first and child routes appear before their parents in
    // the "flattened" version.
    if (route.children && route.children.length > 0) {
      invariant(
      // Our types know better, but runtime JS may not!
      // @ts-expect-error
      route.index !== true, "Index routes must not have child routes. Please remove " + ("all child routes from route path \"" + path + "\"."));
      flattenRoutes(route.children, branches, routesMeta, path);
    }
    // Routes without a path shouldn't ever match by themselves unless they are
    // index routes, so don't add them to the list of possible branches.
    if (route.path == null && !route.index) {
      return;
    }
    branches.push({
      path,
      score: computeScore(path, route.index),
      routesMeta
    });
  };
  routes.forEach((route, index) => {
    var _route$path;
    // coarse-grain check for optional params
    if (route.path === "" || !((_route$path = route.path) != null && _route$path.includes("?"))) {
      flattenRoute(route, index);
    } else {
      for (let exploded of explodeOptionalSegments(route.path)) {
        flattenRoute(route, index, exploded);
      }
    }
  });
  return branches;
}
/**
 * Computes all combinations of optional path segments for a given path,
 * excluding combinations that are ambiguous and of lower priority.
 *
 * For example, `/one/:two?/three/:four?/:five?` explodes to:
 * - `/one/three`
 * - `/one/:two/three`
 * - `/one/three/:four`
 * - `/one/three/:five`
 * - `/one/:two/three/:four`
 * - `/one/:two/three/:five`
 * - `/one/three/:four/:five`
 * - `/one/:two/three/:four/:five`
 */
function explodeOptionalSegments(path) {
  let segments = path.split("/");
  if (segments.length === 0) return [];
  let [first, ...rest] = segments;
  // Optional path segments are denoted by a trailing `?`
  let isOptional = first.endsWith("?");
  // Compute the corresponding required segment: `foo?` -> `foo`
  let required = first.replace(/\?$/, "");
  if (rest.length === 0) {
    // Intepret empty string as omitting an optional segment
    // `["one", "", "three"]` corresponds to omitting `:two` from `/one/:two?/three` -> `/one/three`
    return isOptional ? [required, ""] : [required];
  }
  let restExploded = explodeOptionalSegments(rest.join("/"));
  let result = [];
  // All child paths with the prefix.  Do this for all children before the
  // optional version for all children, so we get consistent ordering where the
  // parent optional aspect is preferred as required.  Otherwise, we can get
  // child sections interspersed where deeper optional segments are higher than
  // parent optional segments, where for example, /:two would explode _earlier_
  // then /:one.  By always including the parent as required _for all children_
  // first, we avoid this issue
  result.push(...restExploded.map(subpath => subpath === "" ? required : [required, subpath].join("/")));
  // Then, if this is an optional value, add all child versions without
  if (isOptional) {
    result.push(...restExploded);
  }
  // for absolute paths, ensure `/` instead of empty segment
  return result.map(exploded => path.startsWith("/") && exploded === "" ? "/" : exploded);
}
function rankRouteBranches(branches) {
  branches.sort((a, b) => a.score !== b.score ? b.score - a.score // Higher score first
  : compareIndexes(a.routesMeta.map(meta => meta.childrenIndex), b.routesMeta.map(meta => meta.childrenIndex)));
}
const paramRe = /^:[\w-]+$/;
const dynamicSegmentValue = 3;
const indexRouteValue = 2;
const emptySegmentValue = 1;
const staticSegmentValue = 10;
const splatPenalty = -2;
const isSplat = s => s === "*";
function computeScore(path, index) {
  let segments = path.split("/");
  let initialScore = segments.length;
  if (segments.some(isSplat)) {
    initialScore += splatPenalty;
  }
  if (index) {
    initialScore += indexRouteValue;
  }
  return segments.filter(s => !isSplat(s)).reduce((score, segment) => score + (paramRe.test(segment) ? dynamicSegmentValue : segment === "" ? emptySegmentValue : staticSegmentValue), initialScore);
}
function compareIndexes(a, b) {
  let siblings = a.length === b.length && a.slice(0, -1).every((n, i) => n === b[i]);
  return siblings ?
  // If two routes are siblings, we should try to match the earlier sibling
  // first. This allows people to have fine-grained control over the matching
  // behavior by simply putting routes with identical paths in the order they
  // want them tried.
  a[a.length - 1] - b[b.length - 1] :
  // Otherwise, it doesn't really make sense to rank non-siblings by index,
  // so they sort equally.
  0;
}
function matchRouteBranch(branch, pathname, allowPartial) {
  if (allowPartial === void 0) {
    allowPartial = false;
  }
  let {
    routesMeta
  } = branch;
  let matchedParams = {};
  let matchedPathname = "/";
  let matches = [];
  for (let i = 0; i < routesMeta.length; ++i) {
    let meta = routesMeta[i];
    let end = i === routesMeta.length - 1;
    let remainingPathname = matchedPathname === "/" ? pathname : pathname.slice(matchedPathname.length) || "/";
    let match = matchPath({
      path: meta.relativePath,
      caseSensitive: meta.caseSensitive,
      end
    }, remainingPathname);
    let route = meta.route;
    if (!match && end && allowPartial && !routesMeta[routesMeta.length - 1].route.index) {
      match = matchPath({
        path: meta.relativePath,
        caseSensitive: meta.caseSensitive,
        end: false
      }, remainingPathname);
    }
    if (!match) {
      return null;
    }
    Object.assign(matchedParams, match.params);
    matches.push({
      // TODO: Can this as be avoided?
      params: matchedParams,
      pathname: joinPaths([matchedPathname, match.pathname]),
      pathnameBase: normalizePathname(joinPaths([matchedPathname, match.pathnameBase])),
      route
    });
    if (match.pathnameBase !== "/") {
      matchedPathname = joinPaths([matchedPathname, match.pathnameBase]);
    }
  }
  return matches;
}
/**
 * Returns a path with params interpolated.
 *
 * @see https://reactrouter.com/v6/utils/generate-path
 */
function generatePath(originalPath, params) {
  if (params === void 0) {
    params = {};
  }
  let path = originalPath;
  if (path.endsWith("*") && path !== "*" && !path.endsWith("/*")) {
    warning(false, "Route path \"" + path + "\" will be treated as if it were " + ("\"" + path.replace(/\*$/, "/*") + "\" because the `*` character must ") + "always follow a `/` in the pattern. To get rid of this warning, " + ("please change the route path to \"" + path.replace(/\*$/, "/*") + "\"."));
    path = path.replace(/\*$/, "/*");
  }
  // ensure `/` is added at the beginning if the path is absolute
  const prefix = path.startsWith("/") ? "/" : "";
  const stringify = p => p == null ? "" : typeof p === "string" ? p : String(p);
  const segments = path.split(/\/+/).map((segment, index, array) => {
    const isLastSegment = index === array.length - 1;
    // only apply the splat if it's the last segment
    if (isLastSegment && segment === "*") {
      const star = "*";
      // Apply the splat
      return stringify(params[star]);
    }
    const keyMatch = segment.match(/^:([\w-]+)(\??)$/);
    if (keyMatch) {
      const [, key, optional] = keyMatch;
      let param = params[key];
      invariant(optional === "?" || param != null, "Missing \":" + key + "\" param");
      return stringify(param);
    }
    // Remove any optional markers from optional static segments
    return segment.replace(/\?$/g, "");
  })
  // Remove empty segments
  .filter(segment => !!segment);
  return prefix + segments.join("/");
}
/**
 * Performs pattern matching on a URL pathname and returns information about
 * the match.
 *
 * @see https://reactrouter.com/v6/utils/match-path
 */
function matchPath(pattern, pathname) {
  if (typeof pattern === "string") {
    pattern = {
      path: pattern,
      caseSensitive: false,
      end: true
    };
  }
  let [matcher, compiledParams] = compilePath(pattern.path, pattern.caseSensitive, pattern.end);
  let match = pathname.match(matcher);
  if (!match) return null;
  let matchedPathname = match[0];
  let pathnameBase = matchedPathname.replace(/(.)\/+$/, "$1");
  let captureGroups = match.slice(1);
  let params = compiledParams.reduce((memo, _ref, index) => {
    let {
      paramName,
      isOptional
    } = _ref;
    // We need to compute the pathnameBase here using the raw splat value
    // instead of using params["*"] later because it will be decoded then
    if (paramName === "*") {
      let splatValue = captureGroups[index] || "";
      pathnameBase = matchedPathname.slice(0, matchedPathname.length - splatValue.length).replace(/(.)\/+$/, "$1");
    }
    const value = captureGroups[index];
    if (isOptional && !value) {
      memo[paramName] = undefined;
    } else {
      memo[paramName] = (value || "").replace(/%2F/g, "/");
    }
    return memo;
  }, {});
  return {
    params,
    pathname: matchedPathname,
    pathnameBase,
    pattern
  };
}
function compilePath(path, caseSensitive, end) {
  if (caseSensitive === void 0) {
    caseSensitive = false;
  }
  if (end === void 0) {
    end = true;
  }
  warning(path === "*" || !path.endsWith("*") || path.endsWith("/*"), "Route path \"" + path + "\" will be treated as if it were " + ("\"" + path.replace(/\*$/, "/*") + "\" because the `*` character must ") + "always follow a `/` in the pattern. To get rid of this warning, " + ("please change the route path to \"" + path.replace(/\*$/, "/*") + "\"."));
  let params = [];
  let regexpSource = "^" + path.replace(/\/*\*?$/, "") // Ignore trailing / and /*, we'll handle it below
  .replace(/^\/*/, "/") // Make sure it has a leading /
  .replace(/[\\.*+^${}|()[\]]/g, "\\$&") // Escape special regex chars
  .replace(/\/:([\w-]+)(\?)?/g, (_, paramName, isOptional) => {
    params.push({
      paramName,
      isOptional: isOptional != null
    });
    return isOptional ? "/?([^\\/]+)?" : "/([^\\/]+)";
  });
  if (path.endsWith("*")) {
    params.push({
      paramName: "*"
    });
    regexpSource += path === "*" || path === "/*" ? "(.*)$" // Already matched the initial /, just match the rest
    : "(?:\\/(.+)|\\/*)$"; // Don't include the / in params["*"]
  } else if (end) {
    // When matching to the end, ignore trailing slashes
    regexpSource += "\\/*$";
  } else if (path !== "" && path !== "/") {
    // If our path is non-empty and contains anything beyond an initial slash,
    // then we have _some_ form of path in our regex, so we should expect to
    // match only if we find the end of this path segment.  Look for an optional
    // non-captured trailing slash (to match a portion of the URL) or the end
    // of the path (if we've matched to the end).  We used to do this with a
    // word boundary but that gives false positives on routes like
    // /user-preferences since `-` counts as a word boundary.
    regexpSource += "(?:(?=\\/|$))";
  } else ;
  let matcher = new RegExp(regexpSource, caseSensitive ? undefined : "i");
  return [matcher, params];
}
function decodePath(value) {
  try {
    return value.split("/").map(v => decodeURIComponent(v).replace(/\//g, "%2F")).join("/");
  } catch (error) {
    warning(false, "The URL path \"" + value + "\" could not be decoded because it is is a " + "malformed URL segment. This is probably due to a bad percent " + ("encoding (" + error + ")."));
    return value;
  }
}
/**
 * @private
 */
function stripBasename(pathname, basename) {
  if (basename === "/") return pathname;
  if (!pathname.toLowerCase().startsWith(basename.toLowerCase())) {
    return null;
  }
  // We want to leave trailing slash behavior in the user's control, so if they
  // specify a basename with a trailing slash, we should support it
  let startIndex = basename.endsWith("/") ? basename.length - 1 : basename.length;
  let nextChar = pathname.charAt(startIndex);
  if (nextChar && nextChar !== "/") {
    // pathname does not start with basename/
    return null;
  }
  return pathname.slice(startIndex) || "/";
}
const ABSOLUTE_URL_REGEX$1 = /^(?:[a-z][a-z0-9+.-]*:|\/\/)/i;
const isAbsoluteUrl = url => ABSOLUTE_URL_REGEX$1.test(url);
/**
 * Returns a resolved path object relative to the given pathname.
 *
 * @see https://reactrouter.com/v6/utils/resolve-path
 */
function resolvePath(to, fromPathname) {
  if (fromPathname === void 0) {
    fromPathname = "/";
  }
  let {
    pathname: toPathname,
    search = "",
    hash = ""
  } = typeof to === "string" ? parsePath(to) : to;
  let pathname;
  if (toPathname) {
    if (isAbsoluteUrl(toPathname)) {
      pathname = toPathname;
    } else {
      if (toPathname.includes("//")) {
        let oldPathname = toPathname;
        toPathname = toPathname.replace(/\/\/+/g, "/");
        warning(false, "Pathnames cannot have embedded double slashes - normalizing " + (oldPathname + " -> " + toPathname));
      }
      if (toPathname.startsWith("/")) {
        pathname = resolvePathname(toPathname.substring(1), "/");
      } else {
        pathname = resolvePathname(toPathname, fromPathname);
      }
    }
  } else {
    pathname = fromPathname;
  }
  return {
    pathname,
    search: normalizeSearch(search),
    hash: normalizeHash(hash)
  };
}
function resolvePathname(relativePath, fromPathname) {
  let segments = fromPathname.replace(/\/+$/, "").split("/");
  let relativeSegments = relativePath.split("/");
  relativeSegments.forEach(segment => {
    if (segment === "..") {
      // Keep the root "" segment so the pathname starts at /
      if (segments.length > 1) segments.pop();
    } else if (segment !== ".") {
      segments.push(segment);
    }
  });
  return segments.length > 1 ? segments.join("/") : "/";
}
function getInvalidPathError(char, field, dest, path) {
  return "Cannot include a '" + char + "' character in a manually specified " + ("`to." + field + "` field [" + JSON.stringify(path) + "].  Please separate it out to the ") + ("`to." + dest + "` field. Alternatively you may provide the full path as ") + "a string in <Link to=\"...\"> and the router will parse it for you.";
}
/**
 * @private
 *
 * When processing relative navigation we want to ignore ancestor routes that
 * do not contribute to the path, such that index/pathless layout routes don't
 * interfere.
 *
 * For example, when moving a route element into an index route and/or a
 * pathless layout route, relative link behavior contained within should stay
 * the same.  Both of the following examples should link back to the root:
 *
 *   <Route path="/">
 *     <Route path="accounts" element={<Link to=".."}>
 *   </Route>
 *
 *   <Route path="/">
 *     <Route path="accounts">
 *       <Route element={<AccountsLayout />}>       // <-- Does not contribute
 *         <Route index element={<Link to=".."} />  // <-- Does not contribute
 *       </Route
 *     </Route>
 *   </Route>
 */
function getPathContributingMatches(matches) {
  return matches.filter((match, index) => index === 0 || match.route.path && match.route.path.length > 0);
}
// Return the array of pathnames for the current route matches - used to
// generate the routePathnames input for resolveTo()
function getResolveToMatches(matches, v7_relativeSplatPath) {
  let pathMatches = getPathContributingMatches(matches);
  // When v7_relativeSplatPath is enabled, use the full pathname for the leaf
  // match so we include splat values for "." links.  See:
  // https://github.com/remix-run/react-router/issues/11052#issuecomment-1836589329
  if (v7_relativeSplatPath) {
    return pathMatches.map((match, idx) => idx === pathMatches.length - 1 ? match.pathname : match.pathnameBase);
  }
  return pathMatches.map(match => match.pathnameBase);
}
/**
 * @private
 */
function resolveTo(toArg, routePathnames, locationPathname, isPathRelative) {
  if (isPathRelative === void 0) {
    isPathRelative = false;
  }
  let to;
  if (typeof toArg === "string") {
    to = parsePath(toArg);
  } else {
    to = _extends$2({}, toArg);
    invariant(!to.pathname || !to.pathname.includes("?"), getInvalidPathError("?", "pathname", "search", to));
    invariant(!to.pathname || !to.pathname.includes("#"), getInvalidPathError("#", "pathname", "hash", to));
    invariant(!to.search || !to.search.includes("#"), getInvalidPathError("#", "search", "hash", to));
  }
  let isEmptyPath = toArg === "" || to.pathname === "";
  let toPathname = isEmptyPath ? "/" : to.pathname;
  let from;
  // Routing is relative to the current pathname if explicitly requested.
  //
  // If a pathname is explicitly provided in `to`, it should be relative to the
  // route context. This is explained in `Note on `<Link to>` values` in our
  // migration guide from v5 as a means of disambiguation between `to` values
  // that begin with `/` and those that do not. However, this is problematic for
  // `to` values that do not provide a pathname. `to` can simply be a search or
  // hash string, in which case we should assume that the navigation is relative
  // to the current location's pathname and *not* the route pathname.
  if (toPathname == null) {
    from = locationPathname;
  } else {
    let routePathnameIndex = routePathnames.length - 1;
    // With relative="route" (the default), each leading .. segment means
    // "go up one route" instead of "go up one URL segment".  This is a key
    // difference from how <a href> works and a major reason we call this a
    // "to" value instead of a "href".
    if (!isPathRelative && toPathname.startsWith("..")) {
      let toSegments = toPathname.split("/");
      while (toSegments[0] === "..") {
        toSegments.shift();
        routePathnameIndex -= 1;
      }
      to.pathname = toSegments.join("/");
    }
    from = routePathnameIndex >= 0 ? routePathnames[routePathnameIndex] : "/";
  }
  let path = resolvePath(to, from);
  // Ensure the pathname has a trailing slash if the original "to" had one
  let hasExplicitTrailingSlash = toPathname && toPathname !== "/" && toPathname.endsWith("/");
  // Or if this was a link to the current path which has a trailing slash
  let hasCurrentTrailingSlash = (isEmptyPath || toPathname === ".") && locationPathname.endsWith("/");
  if (!path.pathname.endsWith("/") && (hasExplicitTrailingSlash || hasCurrentTrailingSlash)) {
    path.pathname += "/";
  }
  return path;
}
/**
 * @private
 */
function getToPathname(to) {
  // Empty strings should be treated the same as / paths
  return to === "" || to.pathname === "" ? "/" : typeof to === "string" ? parsePath(to).pathname : to.pathname;
}
/**
 * @private
 */
const joinPaths = paths => paths.join("/").replace(/\/\/+/g, "/");
/**
 * @private
 */
const normalizePathname = pathname => pathname.replace(/\/+$/, "").replace(/^\/*/, "/");
/**
 * @private
 */
const normalizeSearch = search => !search || search === "?" ? "" : search.startsWith("?") ? search : "?" + search;
/**
 * @private
 */
const normalizeHash = hash => !hash || hash === "#" ? "" : hash.startsWith("#") ? hash : "#" + hash;
/**
 * This is a shortcut for creating `application/json` responses. Converts `data`
 * to JSON and sets the `Content-Type` header.
 *
 * @deprecated The `json` method is deprecated in favor of returning raw objects.
 * This method will be removed in v7.
 */
const json = function json(data, init) {
  if (init === void 0) {
    init = {};
  }
  let responseInit = typeof init === "number" ? {
    status: init
  } : init;
  let headers = new Headers(responseInit.headers);
  if (!headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json; charset=utf-8");
  }
  return new Response(JSON.stringify(data), _extends$2({}, responseInit, {
    headers
  }));
};
class DataWithResponseInit {
  constructor(data, init) {
    this.type = "DataWithResponseInit";
    this.data = data;
    this.init = init || null;
  }
}
/**
 * Create "responses" that contain `status`/`headers` without forcing
 * serialization into an actual `Response` - used by Remix single fetch
 */
function data(data, init) {
  return new DataWithResponseInit(data, typeof init === "number" ? {
    status: init
  } : init);
}
class AbortedDeferredError extends Error {}
class DeferredData {
  constructor(data, responseInit) {
    this.pendingKeysSet = new Set();
    this.subscribers = new Set();
    this.deferredKeys = [];
    invariant(data && typeof data === "object" && !Array.isArray(data), "defer() only accepts plain objects");
    // Set up an AbortController + Promise we can race against to exit early
    // cancellation
    let reject;
    this.abortPromise = new Promise((_, r) => reject = r);
    this.controller = new AbortController();
    let onAbort = () => reject(new AbortedDeferredError("Deferred data aborted"));
    this.unlistenAbortSignal = () => this.controller.signal.removeEventListener("abort", onAbort);
    this.controller.signal.addEventListener("abort", onAbort);
    this.data = Object.entries(data).reduce((acc, _ref2) => {
      let [key, value] = _ref2;
      return Object.assign(acc, {
        [key]: this.trackPromise(key, value)
      });
    }, {});
    if (this.done) {
      // All incoming values were resolved
      this.unlistenAbortSignal();
    }
    this.init = responseInit;
  }
  trackPromise(key, value) {
    if (!(value instanceof Promise)) {
      return value;
    }
    this.deferredKeys.push(key);
    this.pendingKeysSet.add(key);
    // We store a little wrapper promise that will be extended with
    // _data/_error props upon resolve/reject
    let promise = Promise.race([value, this.abortPromise]).then(data => this.onSettle(promise, key, undefined, data), error => this.onSettle(promise, key, error));
    // Register rejection listeners to avoid uncaught promise rejections on
    // errors or aborted deferred values
    promise.catch(() => {});
    Object.defineProperty(promise, "_tracked", {
      get: () => true
    });
    return promise;
  }
  onSettle(promise, key, error, data) {
    if (this.controller.signal.aborted && error instanceof AbortedDeferredError) {
      this.unlistenAbortSignal();
      Object.defineProperty(promise, "_error", {
        get: () => error
      });
      return Promise.reject(error);
    }
    this.pendingKeysSet.delete(key);
    if (this.done) {
      // Nothing left to abort!
      this.unlistenAbortSignal();
    }
    // If the promise was resolved/rejected with undefined, we'll throw an error as you
    // should always resolve with a value or null
    if (error === undefined && data === undefined) {
      let undefinedError = new Error("Deferred data for key \"" + key + "\" resolved/rejected with `undefined`, " + "you must resolve/reject with a value or `null`.");
      Object.defineProperty(promise, "_error", {
        get: () => undefinedError
      });
      this.emit(false, key);
      return Promise.reject(undefinedError);
    }
    if (data === undefined) {
      Object.defineProperty(promise, "_error", {
        get: () => error
      });
      this.emit(false, key);
      return Promise.reject(error);
    }
    Object.defineProperty(promise, "_data", {
      get: () => data
    });
    this.emit(false, key);
    return data;
  }
  emit(aborted, settledKey) {
    this.subscribers.forEach(subscriber => subscriber(aborted, settledKey));
  }
  subscribe(fn) {
    this.subscribers.add(fn);
    return () => this.subscribers.delete(fn);
  }
  cancel() {
    this.controller.abort();
    this.pendingKeysSet.forEach((v, k) => this.pendingKeysSet.delete(k));
    this.emit(true);
  }
  async resolveData(signal) {
    let aborted = false;
    if (!this.done) {
      let onAbort = () => this.cancel();
      signal.addEventListener("abort", onAbort);
      aborted = await new Promise(resolve => {
        this.subscribe(aborted => {
          signal.removeEventListener("abort", onAbort);
          if (aborted || this.done) {
            resolve(aborted);
          }
        });
      });
    }
    return aborted;
  }
  get done() {
    return this.pendingKeysSet.size === 0;
  }
  get unwrappedData() {
    invariant(this.data !== null && this.done, "Can only unwrap data on initialized and settled deferreds");
    return Object.entries(this.data).reduce((acc, _ref3) => {
      let [key, value] = _ref3;
      return Object.assign(acc, {
        [key]: unwrapTrackedPromise(value)
      });
    }, {});
  }
  get pendingKeys() {
    return Array.from(this.pendingKeysSet);
  }
}
function isTrackedPromise(value) {
  return value instanceof Promise && value._tracked === true;
}
function unwrapTrackedPromise(value) {
  if (!isTrackedPromise(value)) {
    return value;
  }
  if (value._error) {
    throw value._error;
  }
  return value._data;
}
/**
 * @deprecated The `defer` method is deprecated in favor of returning raw
 * objects. This method will be removed in v7.
 */
const defer = function defer(data, init) {
  if (init === void 0) {
    init = {};
  }
  let responseInit = typeof init === "number" ? {
    status: init
  } : init;
  return new DeferredData(data, responseInit);
};
/**
 * A redirect response. Sets the status code and the `Location` header.
 * Defaults to "302 Found".
 */
const redirect = function redirect(url, init) {
  if (init === void 0) {
    init = 302;
  }
  let responseInit = init;
  if (typeof responseInit === "number") {
    responseInit = {
      status: responseInit
    };
  } else if (typeof responseInit.status === "undefined") {
    responseInit.status = 302;
  }
  let headers = new Headers(responseInit.headers);
  headers.set("Location", url);
  return new Response(null, _extends$2({}, responseInit, {
    headers
  }));
};
/**
 * A redirect response that will force a document reload to the new location.
 * Sets the status code and the `Location` header.
 * Defaults to "302 Found".
 */
const redirectDocument = (url, init) => {
  let response = redirect(url, init);
  response.headers.set("X-Remix-Reload-Document", "true");
  return response;
};
/**
 * A redirect response that will perform a `history.replaceState` instead of a
 * `history.pushState` for client-side navigation redirects.
 * Sets the status code and the `Location` header.
 * Defaults to "302 Found".
 */
const replace = (url, init) => {
  let response = redirect(url, init);
  response.headers.set("X-Remix-Replace", "true");
  return response;
};
/**
 * @private
 * Utility class we use to hold auto-unwrapped 4xx/5xx Response bodies
 *
 * We don't export the class for public use since it's an implementation
 * detail, but we export the interface above so folks can build their own
 * abstractions around instances via isRouteErrorResponse()
 */
class ErrorResponseImpl {
  constructor(status, statusText, data, internal) {
    if (internal === void 0) {
      internal = false;
    }
    this.status = status;
    this.statusText = statusText || "";
    this.internal = internal;
    if (data instanceof Error) {
      this.data = data.toString();
      this.error = data;
    } else {
      this.data = data;
    }
  }
}
/**
 * Check if the given error is an ErrorResponse generated from a 4xx/5xx
 * Response thrown from an action/loader
 */
function isRouteErrorResponse(error) {
  return error != null && typeof error.status === "number" && typeof error.statusText === "string" && typeof error.internal === "boolean" && "data" in error;
}

const validMutationMethodsArr = ["post", "put", "patch", "delete"];
const validMutationMethods = new Set(validMutationMethodsArr);
const validRequestMethodsArr = ["get", ...validMutationMethodsArr];
const validRequestMethods = new Set(validRequestMethodsArr);
const redirectStatusCodes = new Set([301, 302, 303, 307, 308]);
const redirectPreserveMethodStatusCodes = new Set([307, 308]);
const IDLE_NAVIGATION = {
  state: "idle",
  location: undefined,
  formMethod: undefined,
  formAction: undefined,
  formEncType: undefined,
  formData: undefined,
  json: undefined,
  text: undefined
};
const IDLE_FETCHER = {
  state: "idle",
  data: undefined,
  formMethod: undefined,
  formAction: undefined,
  formEncType: undefined,
  formData: undefined,
  json: undefined,
  text: undefined
};
const IDLE_BLOCKER = {
  state: "unblocked",
  proceed: undefined,
  reset: undefined,
  location: undefined
};
const ABSOLUTE_URL_REGEX$2 = /^(?:[a-z][a-z0-9+.-]*:|\/\/)/i;
const defaultMapRouteProperties = route => ({
  hasErrorBoundary: Boolean(route.hasErrorBoundary)
});
const TRANSITIONS_STORAGE_KEY = "remix-router-transitions";
//#endregion
////////////////////////////////////////////////////////////////////////////////
//#region createRouter
////////////////////////////////////////////////////////////////////////////////
/**
 * Create a router and listen to history POP navigations
 */
function createRouter(init) {
  const routerWindow = init.window ? init.window : typeof window !== "undefined" ? window : undefined;
  const isBrowser = typeof routerWindow !== "undefined" && typeof routerWindow.document !== "undefined" && typeof routerWindow.document.createElement !== "undefined";
  const isServer = !isBrowser;
  invariant(init.routes.length > 0, "You must provide a non-empty routes array to createRouter");
  let mapRouteProperties;
  if (init.mapRouteProperties) {
    mapRouteProperties = init.mapRouteProperties;
  } else if (init.detectErrorBoundary) {
    // If they are still using the deprecated version, wrap it with the new API
    let detectErrorBoundary = init.detectErrorBoundary;
    mapRouteProperties = route => ({
      hasErrorBoundary: detectErrorBoundary(route)
    });
  } else {
    mapRouteProperties = defaultMapRouteProperties;
  }
  // Routes keyed by ID
  let manifest = {};
  // Routes in tree format for matching
  let dataRoutes = convertRoutesToDataRoutes(init.routes, mapRouteProperties, undefined, manifest);
  let inFlightDataRoutes;
  let basename = init.basename || "/";
  let dataStrategyImpl = init.dataStrategy || defaultDataStrategy;
  let patchRoutesOnNavigationImpl = init.patchRoutesOnNavigation;
  // Config driven behavior flags
  let future = _extends$2({
    v7_fetcherPersist: false,
    v7_normalizeFormMethod: false,
    v7_partialHydration: false,
    v7_prependBasename: false,
    v7_relativeSplatPath: false,
    v7_skipActionErrorRevalidation: false
  }, init.future);
  // Cleanup function for history
  let unlistenHistory = null;
  // Externally-provided functions to call on all state changes
  let subscribers = new Set();
  // Externally-provided object to hold scroll restoration locations during routing
  let savedScrollPositions = null;
  // Externally-provided function to get scroll restoration keys
  let getScrollRestorationKey = null;
  // Externally-provided function to get current scroll position
  let getScrollPosition = null;
  // One-time flag to control the initial hydration scroll restoration.  Because
  // we don't get the saved positions from <ScrollRestoration /> until _after_
  // the initial render, we need to manually trigger a separate updateState to
  // send along the restoreScrollPosition
  // Set to true if we have `hydrationData` since we assume we were SSR'd and that
  // SSR did the initial scroll restoration.
  let initialScrollRestored = init.hydrationData != null;
  let initialMatches = matchRoutes(dataRoutes, init.history.location, basename);
  let initialMatchesIsFOW = false;
  let initialErrors = null;
  if (initialMatches == null && !patchRoutesOnNavigationImpl) {
    // If we do not match a user-provided-route, fall back to the root
    // to allow the error boundary to take over
    let error = getInternalRouterError(404, {
      pathname: init.history.location.pathname
    });
    let {
      matches,
      route
    } = getShortCircuitMatches(dataRoutes);
    initialMatches = matches;
    initialErrors = {
      [route.id]: error
    };
  }
  // In SPA apps, if the user provided a patchRoutesOnNavigation implementation and
  // our initial match is a splat route, clear them out so we run through lazy
  // discovery on hydration in case there's a more accurate lazy route match.
  // In SSR apps (with `hydrationData`), we expect that the server will send
  // up the proper matched routes so we don't want to run lazy discovery on
  // initial hydration and want to hydrate into the splat route.
  if (initialMatches && !init.hydrationData) {
    let fogOfWar = checkFogOfWar(initialMatches, dataRoutes, init.history.location.pathname);
    if (fogOfWar.active) {
      initialMatches = null;
    }
  }
  let initialized;
  if (!initialMatches) {
    initialized = false;
    initialMatches = [];
    // If partial hydration and fog of war is enabled, we will be running
    // `patchRoutesOnNavigation` during hydration so include any partial matches as
    // the initial matches so we can properly render `HydrateFallback`'s
    if (future.v7_partialHydration) {
      let fogOfWar = checkFogOfWar(null, dataRoutes, init.history.location.pathname);
      if (fogOfWar.active && fogOfWar.matches) {
        initialMatchesIsFOW = true;
        initialMatches = fogOfWar.matches;
      }
    }
  } else if (initialMatches.some(m => m.route.lazy)) {
    // All initialMatches need to be loaded before we're ready.  If we have lazy
    // functions around still then we'll need to run them in initialize()
    initialized = false;
  } else if (!initialMatches.some(m => m.route.loader)) {
    // If we've got no loaders to run, then we're good to go
    initialized = true;
  } else if (future.v7_partialHydration) {
    // If partial hydration is enabled, we're initialized so long as we were
    // provided with hydrationData for every route with a loader, and no loaders
    // were marked for explicit hydration
    let loaderData = init.hydrationData ? init.hydrationData.loaderData : null;
    let errors = init.hydrationData ? init.hydrationData.errors : null;
    // If errors exist, don't consider routes below the boundary
    if (errors) {
      let idx = initialMatches.findIndex(m => errors[m.route.id] !== undefined);
      initialized = initialMatches.slice(0, idx + 1).every(m => !shouldLoadRouteOnHydration(m.route, loaderData, errors));
    } else {
      initialized = initialMatches.every(m => !shouldLoadRouteOnHydration(m.route, loaderData, errors));
    }
  } else {
    // Without partial hydration - we're initialized if we were provided any
    // hydrationData - which is expected to be complete
    initialized = init.hydrationData != null;
  }
  let router;
  let state = {
    historyAction: init.history.action,
    location: init.history.location,
    matches: initialMatches,
    initialized,
    navigation: IDLE_NAVIGATION,
    // Don't restore on initial updateState() if we were SSR'd
    restoreScrollPosition: init.hydrationData != null ? false : null,
    preventScrollReset: false,
    revalidation: "idle",
    loaderData: init.hydrationData && init.hydrationData.loaderData || {},
    actionData: init.hydrationData && init.hydrationData.actionData || null,
    errors: init.hydrationData && init.hydrationData.errors || initialErrors,
    fetchers: new Map(),
    blockers: new Map()
  };
  // -- Stateful internal variables to manage navigations --
  // Current navigation in progress (to be committed in completeNavigation)
  let pendingAction = Action.Pop;
  // Should the current navigation prevent the scroll reset if scroll cannot
  // be restored?
  let pendingPreventScrollReset = false;
  // AbortController for the active navigation
  let pendingNavigationController;
  // Should the current navigation enable document.startViewTransition?
  let pendingViewTransitionEnabled = false;
  // Store applied view transitions so we can apply them on POP
  let appliedViewTransitions = new Map();
  // Cleanup function for persisting applied transitions to sessionStorage
  let removePageHideEventListener = null;
  // We use this to avoid touching history in completeNavigation if a
  // revalidation is entirely uninterrupted
  let isUninterruptedRevalidation = false;
  // Use this internal flag to force revalidation of all loaders:
  //  - submissions (completed or interrupted)
  //  - useRevalidator()
  //  - X-Remix-Revalidate (from redirect)
  let isRevalidationRequired = false;
  // Use this internal array to capture routes that require revalidation due
  // to a cancelled deferred on action submission
  let cancelledDeferredRoutes = [];
  // Use this internal array to capture fetcher loads that were cancelled by an
  // action navigation and require revalidation
  let cancelledFetcherLoads = new Set();
  // AbortControllers for any in-flight fetchers
  let fetchControllers = new Map();
  // Track loads based on the order in which they started
  let incrementingLoadId = 0;
  // Track the outstanding pending navigation data load to be compared against
  // the globally incrementing load when a fetcher load lands after a completed
  // navigation
  let pendingNavigationLoadId = -1;
  // Fetchers that triggered data reloads as a result of their actions
  let fetchReloadIds = new Map();
  // Fetchers that triggered redirect navigations
  let fetchRedirectIds = new Set();
  // Most recent href/match for fetcher.load calls for fetchers
  let fetchLoadMatches = new Map();
  // Ref-count mounted fetchers so we know when it's ok to clean them up
  let activeFetchers = new Map();
  // Fetchers that have requested a delete when using v7_fetcherPersist,
  // they'll be officially removed after they return to idle
  let deletedFetchers = new Set();
  // Store DeferredData instances for active route matches.  When a
  // route loader returns defer() we stick one in here.  Then, when a nested
  // promise resolves we update loaderData.  If a new navigation starts we
  // cancel active deferreds for eliminated routes.
  let activeDeferreds = new Map();
  // Store blocker functions in a separate Map outside of router state since
  // we don't need to update UI state if they change
  let blockerFunctions = new Map();
  // Flag to ignore the next history update, so we can revert the URL change on
  // a POP navigation that was blocked by the user without touching router state
  let unblockBlockerHistoryUpdate = undefined;
  // Initialize the router, all side effects should be kicked off from here.
  // Implemented as a Fluent API for ease of:
  //   let router = createRouter(init).initialize();
  function initialize() {
    // If history informs us of a POP navigation, start the navigation but do not update
    // state.  We'll update our own state once the navigation completes
    unlistenHistory = init.history.listen(_ref => {
      let {
        action: historyAction,
        location,
        delta
      } = _ref;
      // Ignore this event if it was just us resetting the URL from a
      // blocked POP navigation
      if (unblockBlockerHistoryUpdate) {
        unblockBlockerHistoryUpdate();
        unblockBlockerHistoryUpdate = undefined;
        return;
      }
      warning(blockerFunctions.size === 0 || delta != null, "You are trying to use a blocker on a POP navigation to a location " + "that was not created by @remix-run/router. This will fail silently in " + "production. This can happen if you are navigating outside the router " + "via `window.history.pushState`/`window.location.hash` instead of using " + "router navigation APIs.  This can also happen if you are using " + "createHashRouter and the user manually changes the URL.");
      let blockerKey = shouldBlockNavigation({
        currentLocation: state.location,
        nextLocation: location,
        historyAction
      });
      if (blockerKey && delta != null) {
        // Restore the URL to match the current UI, but don't update router state
        let nextHistoryUpdatePromise = new Promise(resolve => {
          unblockBlockerHistoryUpdate = resolve;
        });
        init.history.go(delta * -1);
        // Put the blocker into a blocked state
        updateBlocker(blockerKey, {
          state: "blocked",
          location,
          proceed() {
            updateBlocker(blockerKey, {
              state: "proceeding",
              proceed: undefined,
              reset: undefined,
              location
            });
            // Re-do the same POP navigation we just blocked, after the url
            // restoration is also complete.  See:
            // https://github.com/remix-run/react-router/issues/11613
            nextHistoryUpdatePromise.then(() => init.history.go(delta));
          },
          reset() {
            let blockers = new Map(state.blockers);
            blockers.set(blockerKey, IDLE_BLOCKER);
            updateState({
              blockers
            });
          }
        });
        return;
      }
      return startNavigation(historyAction, location);
    });
    if (isBrowser) {
      // FIXME: This feels gross.  How can we cleanup the lines between
      // scrollRestoration/appliedTransitions persistance?
      restoreAppliedTransitions(routerWindow, appliedViewTransitions);
      let _saveAppliedTransitions = () => persistAppliedTransitions(routerWindow, appliedViewTransitions);
      routerWindow.addEventListener("pagehide", _saveAppliedTransitions);
      removePageHideEventListener = () => routerWindow.removeEventListener("pagehide", _saveAppliedTransitions);
    }
    // Kick off initial data load if needed.  Use Pop to avoid modifying history
    // Note we don't do any handling of lazy here.  For SPA's it'll get handled
    // in the normal navigation flow.  For SSR it's expected that lazy modules are
    // resolved prior to router creation since we can't go into a fallbackElement
    // UI for SSR'd apps
    if (!state.initialized) {
      startNavigation(Action.Pop, state.location, {
        initialHydration: true
      });
    }
    return router;
  }
  // Clean up a router and it's side effects
  function dispose() {
    if (unlistenHistory) {
      unlistenHistory();
    }
    if (removePageHideEventListener) {
      removePageHideEventListener();
    }
    subscribers.clear();
    pendingNavigationController && pendingNavigationController.abort();
    state.fetchers.forEach((_, key) => deleteFetcher(key));
    state.blockers.forEach((_, key) => deleteBlocker(key));
  }
  // Subscribe to state updates for the router
  function subscribe(fn) {
    subscribers.add(fn);
    return () => subscribers.delete(fn);
  }
  // Update our state and notify the calling context of the change
  function updateState(newState, opts) {
    if (opts === void 0) {
      opts = {};
    }
    state = _extends$2({}, state, newState);
    // Prep fetcher cleanup so we can tell the UI which fetcher data entries
    // can be removed
    let completedFetchers = [];
    let deletedFetchersKeys = [];
    if (future.v7_fetcherPersist) {
      state.fetchers.forEach((fetcher, key) => {
        if (fetcher.state === "idle") {
          if (deletedFetchers.has(key)) {
            // Unmounted from the UI and can be totally removed
            deletedFetchersKeys.push(key);
          } else {
            // Returned to idle but still mounted in the UI, so semi-remains for
            // revalidations and such
            completedFetchers.push(key);
          }
        }
      });
    }
    // Remove any lingering deleted fetchers that have already been removed
    // from state.fetchers
    deletedFetchers.forEach(key => {
      if (!state.fetchers.has(key) && !fetchControllers.has(key)) {
        deletedFetchersKeys.push(key);
      }
    });
    // Iterate over a local copy so that if flushSync is used and we end up
    // removing and adding a new subscriber due to the useCallback dependencies,
    // we don't get ourselves into a loop calling the new subscriber immediately
    [...subscribers].forEach(subscriber => subscriber(state, {
      deletedFetchers: deletedFetchersKeys,
      viewTransitionOpts: opts.viewTransitionOpts,
      flushSync: opts.flushSync === true
    }));
    // Remove idle fetchers from state since we only care about in-flight fetchers.
    if (future.v7_fetcherPersist) {
      completedFetchers.forEach(key => state.fetchers.delete(key));
      deletedFetchersKeys.forEach(key => deleteFetcher(key));
    } else {
      // We already called deleteFetcher() on these, can remove them from this
      // Set now that we've handed the keys off to the data layer
      deletedFetchersKeys.forEach(key => deletedFetchers.delete(key));
    }
  }
  // Complete a navigation returning the state.navigation back to the IDLE_NAVIGATION
  // and setting state.[historyAction/location/matches] to the new route.
  // - Location is a required param
  // - Navigation will always be set to IDLE_NAVIGATION
  // - Can pass any other state in newState
  function completeNavigation(location, newState, _temp) {
    var _location$state, _location$state2;
    let {
      flushSync
    } = _temp === void 0 ? {} : _temp;
    // Deduce if we're in a loading/actionReload state:
    // - We have committed actionData in the store
    // - The current navigation was a mutation submission
    // - We're past the submitting state and into the loading state
    // - The location being loaded is not the result of a redirect
    let isActionReload = state.actionData != null && state.navigation.formMethod != null && isMutationMethod(state.navigation.formMethod) && state.navigation.state === "loading" && ((_location$state = location.state) == null ? void 0 : _location$state._isRedirect) !== true;
    let actionData;
    if (newState.actionData) {
      if (Object.keys(newState.actionData).length > 0) {
        actionData = newState.actionData;
      } else {
        // Empty actionData -> clear prior actionData due to an action error
        actionData = null;
      }
    } else if (isActionReload) {
      // Keep the current data if we're wrapping up the action reload
      actionData = state.actionData;
    } else {
      // Clear actionData on any other completed navigations
      actionData = null;
    }
    // Always preserve any existing loaderData from re-used routes
    let loaderData = newState.loaderData ? mergeLoaderData(state.loaderData, newState.loaderData, newState.matches || [], newState.errors) : state.loaderData;
    // On a successful navigation we can assume we got through all blockers
    // so we can start fresh
    let blockers = state.blockers;
    if (blockers.size > 0) {
      blockers = new Map(blockers);
      blockers.forEach((_, k) => blockers.set(k, IDLE_BLOCKER));
    }
    // Always respect the user flag.  Otherwise don't reset on mutation
    // submission navigations unless they redirect
    let preventScrollReset = pendingPreventScrollReset === true || state.navigation.formMethod != null && isMutationMethod(state.navigation.formMethod) && ((_location$state2 = location.state) == null ? void 0 : _location$state2._isRedirect) !== true;
    // Commit any in-flight routes at the end of the HMR revalidation "navigation"
    if (inFlightDataRoutes) {
      dataRoutes = inFlightDataRoutes;
      inFlightDataRoutes = undefined;
    }
    if (isUninterruptedRevalidation) ; else if (pendingAction === Action.Pop) ; else if (pendingAction === Action.Push) {
      init.history.push(location, location.state);
    } else if (pendingAction === Action.Replace) {
      init.history.replace(location, location.state);
    }
    let viewTransitionOpts;
    // On POP, enable transitions if they were enabled on the original navigation
    if (pendingAction === Action.Pop) {
      // Forward takes precedence so they behave like the original navigation
      let priorPaths = appliedViewTransitions.get(state.location.pathname);
      if (priorPaths && priorPaths.has(location.pathname)) {
        viewTransitionOpts = {
          currentLocation: state.location,
          nextLocation: location
        };
      } else if (appliedViewTransitions.has(location.pathname)) {
        // If we don't have a previous forward nav, assume we're popping back to
        // the new location and enable if that location previously enabled
        viewTransitionOpts = {
          currentLocation: location,
          nextLocation: state.location
        };
      }
    } else if (pendingViewTransitionEnabled) {
      // Store the applied transition on PUSH/REPLACE
      let toPaths = appliedViewTransitions.get(state.location.pathname);
      if (toPaths) {
        toPaths.add(location.pathname);
      } else {
        toPaths = new Set([location.pathname]);
        appliedViewTransitions.set(state.location.pathname, toPaths);
      }
      viewTransitionOpts = {
        currentLocation: state.location,
        nextLocation: location
      };
    }
    updateState(_extends$2({}, newState, {
      actionData,
      loaderData,
      historyAction: pendingAction,
      location,
      initialized: true,
      navigation: IDLE_NAVIGATION,
      revalidation: "idle",
      restoreScrollPosition: getSavedScrollPosition(location, newState.matches || state.matches),
      preventScrollReset,
      blockers
    }), {
      viewTransitionOpts,
      flushSync: flushSync === true
    });
    // Reset stateful navigation vars
    pendingAction = Action.Pop;
    pendingPreventScrollReset = false;
    pendingViewTransitionEnabled = false;
    isUninterruptedRevalidation = false;
    isRevalidationRequired = false;
    cancelledDeferredRoutes = [];
  }
  // Trigger a navigation event, which can either be a numerical POP or a PUSH
  // replace with an optional submission
  async function navigate(to, opts) {
    if (typeof to === "number") {
      init.history.go(to);
      return;
    }
    let normalizedPath = normalizeTo(state.location, state.matches, basename, future.v7_prependBasename, to, future.v7_relativeSplatPath, opts == null ? void 0 : opts.fromRouteId, opts == null ? void 0 : opts.relative);
    let {
      path,
      submission,
      error
    } = normalizeNavigateOptions(future.v7_normalizeFormMethod, false, normalizedPath, opts);
    let currentLocation = state.location;
    let nextLocation = createLocation(state.location, path, opts && opts.state);
    // When using navigate as a PUSH/REPLACE we aren't reading an already-encoded
    // URL from window.location, so we need to encode it here so the behavior
    // remains the same as POP and non-data-router usages.  new URL() does all
    // the same encoding we'd get from a history.pushState/window.location read
    // without having to touch history
    nextLocation = _extends$2({}, nextLocation, init.history.encodeLocation(nextLocation));
    let userReplace = opts && opts.replace != null ? opts.replace : undefined;
    let historyAction = Action.Push;
    if (userReplace === true) {
      historyAction = Action.Replace;
    } else if (userReplace === false) ; else if (submission != null && isMutationMethod(submission.formMethod) && submission.formAction === state.location.pathname + state.location.search) {
      // By default on submissions to the current location we REPLACE so that
      // users don't have to double-click the back button to get to the prior
      // location.  If the user redirects to a different location from the
      // action/loader this will be ignored and the redirect will be a PUSH
      historyAction = Action.Replace;
    }
    let preventScrollReset = opts && "preventScrollReset" in opts ? opts.preventScrollReset === true : undefined;
    let flushSync = (opts && opts.flushSync) === true;
    let blockerKey = shouldBlockNavigation({
      currentLocation,
      nextLocation,
      historyAction
    });
    if (blockerKey) {
      // Put the blocker into a blocked state
      updateBlocker(blockerKey, {
        state: "blocked",
        location: nextLocation,
        proceed() {
          updateBlocker(blockerKey, {
            state: "proceeding",
            proceed: undefined,
            reset: undefined,
            location: nextLocation
          });
          // Send the same navigation through
          navigate(to, opts);
        },
        reset() {
          let blockers = new Map(state.blockers);
          blockers.set(blockerKey, IDLE_BLOCKER);
          updateState({
            blockers
          });
        }
      });
      return;
    }
    return await startNavigation(historyAction, nextLocation, {
      submission,
      // Send through the formData serialization error if we have one so we can
      // render at the right error boundary after we match routes
      pendingError: error,
      preventScrollReset,
      replace: opts && opts.replace,
      enableViewTransition: opts && opts.viewTransition,
      flushSync
    });
  }
  // Revalidate all current loaders.  If a navigation is in progress or if this
  // is interrupted by a navigation, allow this to "succeed" by calling all
  // loaders during the next loader round
  function revalidate() {
    interruptActiveLoads();
    updateState({
      revalidation: "loading"
    });
    // If we're currently submitting an action, we don't need to start a new
    // navigation, we'll just let the follow up loader execution call all loaders
    if (state.navigation.state === "submitting") {
      return;
    }
    // If we're currently in an idle state, start a new navigation for the current
    // action/location and mark it as uninterrupted, which will skip the history
    // update in completeNavigation
    if (state.navigation.state === "idle") {
      startNavigation(state.historyAction, state.location, {
        startUninterruptedRevalidation: true
      });
      return;
    }
    // Otherwise, if we're currently in a loading state, just start a new
    // navigation to the navigation.location but do not trigger an uninterrupted
    // revalidation so that history correctly updates once the navigation completes
    startNavigation(pendingAction || state.historyAction, state.navigation.location, {
      overrideNavigation: state.navigation,
      // Proxy through any rending view transition
      enableViewTransition: pendingViewTransitionEnabled === true
    });
  }
  // Start a navigation to the given action/location.  Can optionally provide a
  // overrideNavigation which will override the normalLoad in the case of a redirect
  // navigation
  async function startNavigation(historyAction, location, opts) {
    // Abort any in-progress navigations and start a new one. Unset any ongoing
    // uninterrupted revalidations unless told otherwise, since we want this
    // new navigation to update history normally
    pendingNavigationController && pendingNavigationController.abort();
    pendingNavigationController = null;
    pendingAction = historyAction;
    isUninterruptedRevalidation = (opts && opts.startUninterruptedRevalidation) === true;
    // Save the current scroll position every time we start a new navigation,
    // and track whether we should reset scroll on completion
    saveScrollPosition(state.location, state.matches);
    pendingPreventScrollReset = (opts && opts.preventScrollReset) === true;
    pendingViewTransitionEnabled = (opts && opts.enableViewTransition) === true;
    let routesToUse = inFlightDataRoutes || dataRoutes;
    let loadingNavigation = opts && opts.overrideNavigation;
    let matches = opts != null && opts.initialHydration && state.matches && state.matches.length > 0 && !initialMatchesIsFOW ?
    // `matchRoutes()` has already been called if we're in here via `router.initialize()`
    state.matches : matchRoutes(routesToUse, location, basename);
    let flushSync = (opts && opts.flushSync) === true;
    // Short circuit if it's only a hash change and not a revalidation or
    // mutation submission.
    //
    // Ignore on initial page loads because since the initial hydration will always
    // be "same hash".  For example, on /page#hash and submit a <Form method="post">
    // which will default to a navigation to /page
    if (matches && state.initialized && !isRevalidationRequired && isHashChangeOnly(state.location, location) && !(opts && opts.submission && isMutationMethod(opts.submission.formMethod))) {
      completeNavigation(location, {
        matches
      }, {
        flushSync
      });
      return;
    }
    let fogOfWar = checkFogOfWar(matches, routesToUse, location.pathname);
    if (fogOfWar.active && fogOfWar.matches) {
      matches = fogOfWar.matches;
    }
    // Short circuit with a 404 on the root error boundary if we match nothing
    if (!matches) {
      let {
        error,
        notFoundMatches,
        route
      } = handleNavigational404(location.pathname);
      completeNavigation(location, {
        matches: notFoundMatches,
        loaderData: {},
        errors: {
          [route.id]: error
        }
      }, {
        flushSync
      });
      return;
    }
    // Create a controller/Request for this navigation
    pendingNavigationController = new AbortController();
    let request = createClientSideRequest(init.history, location, pendingNavigationController.signal, opts && opts.submission);
    let pendingActionResult;
    if (opts && opts.pendingError) {
      // If we have a pendingError, it means the user attempted a GET submission
      // with binary FormData so assign here and skip to handleLoaders.  That
      // way we handle calling loaders above the boundary etc.  It's not really
      // different from an actionError in that sense.
      pendingActionResult = [findNearestBoundary(matches).route.id, {
        type: ResultType.error,
        error: opts.pendingError
      }];
    } else if (opts && opts.submission && isMutationMethod(opts.submission.formMethod)) {
      // Call action if we received an action submission
      let actionResult = await handleAction(request, location, opts.submission, matches, fogOfWar.active, {
        replace: opts.replace,
        flushSync
      });
      if (actionResult.shortCircuited) {
        return;
      }
      // If we received a 404 from handleAction, it's because we couldn't lazily
      // discover the destination route so we don't want to call loaders
      if (actionResult.pendingActionResult) {
        let [routeId, result] = actionResult.pendingActionResult;
        if (isErrorResult(result) && isRouteErrorResponse(result.error) && result.error.status === 404) {
          pendingNavigationController = null;
          completeNavigation(location, {
            matches: actionResult.matches,
            loaderData: {},
            errors: {
              [routeId]: result.error
            }
          });
          return;
        }
      }
      matches = actionResult.matches || matches;
      pendingActionResult = actionResult.pendingActionResult;
      loadingNavigation = getLoadingNavigation(location, opts.submission);
      flushSync = false;
      // No need to do fog of war matching again on loader execution
      fogOfWar.active = false;
      // Create a GET request for the loaders
      request = createClientSideRequest(init.history, request.url, request.signal);
    }
    // Call loaders
    let {
      shortCircuited,
      matches: updatedMatches,
      loaderData,
      errors
    } = await handleLoaders(request, location, matches, fogOfWar.active, loadingNavigation, opts && opts.submission, opts && opts.fetcherSubmission, opts && opts.replace, opts && opts.initialHydration === true, flushSync, pendingActionResult);
    if (shortCircuited) {
      return;
    }
    // Clean up now that the action/loaders have completed.  Don't clean up if
    // we short circuited because pendingNavigationController will have already
    // been assigned to a new controller for the next navigation
    pendingNavigationController = null;
    completeNavigation(location, _extends$2({
      matches: updatedMatches || matches
    }, getActionDataForCommit(pendingActionResult), {
      loaderData,
      errors
    }));
  }
  // Call the action matched by the leaf route for this navigation and handle
  // redirects/errors
  async function handleAction(request, location, submission, matches, isFogOfWar, opts) {
    if (opts === void 0) {
      opts = {};
    }
    interruptActiveLoads();
    // Put us in a submitting state
    let navigation = getSubmittingNavigation(location, submission);
    updateState({
      navigation
    }, {
      flushSync: opts.flushSync === true
    });
    if (isFogOfWar) {
      let discoverResult = await discoverRoutes(matches, location.pathname, request.signal);
      if (discoverResult.type === "aborted") {
        return {
          shortCircuited: true
        };
      } else if (discoverResult.type === "error") {
        let boundaryId = findNearestBoundary(discoverResult.partialMatches).route.id;
        return {
          matches: discoverResult.partialMatches,
          pendingActionResult: [boundaryId, {
            type: ResultType.error,
            error: discoverResult.error
          }]
        };
      } else if (!discoverResult.matches) {
        let {
          notFoundMatches,
          error,
          route
        } = handleNavigational404(location.pathname);
        return {
          matches: notFoundMatches,
          pendingActionResult: [route.id, {
            type: ResultType.error,
            error
          }]
        };
      } else {
        matches = discoverResult.matches;
      }
    }
    // Call our action and get the result
    let result;
    let actionMatch = getTargetMatch(matches, location);
    if (!actionMatch.route.action && !actionMatch.route.lazy) {
      result = {
        type: ResultType.error,
        error: getInternalRouterError(405, {
          method: request.method,
          pathname: location.pathname,
          routeId: actionMatch.route.id
        })
      };
    } else {
      let results = await callDataStrategy("action", state, request, [actionMatch], matches, null);
      result = results[actionMatch.route.id];
      if (request.signal.aborted) {
        return {
          shortCircuited: true
        };
      }
    }
    if (isRedirectResult(result)) {
      let replace;
      if (opts && opts.replace != null) {
        replace = opts.replace;
      } else {
        // If the user didn't explicity indicate replace behavior, replace if
        // we redirected to the exact same location we're currently at to avoid
        // double back-buttons
        let location = normalizeRedirectLocation(result.response.headers.get("Location"), new URL(request.url), basename);
        replace = location === state.location.pathname + state.location.search;
      }
      await startRedirectNavigation(request, result, true, {
        submission,
        replace
      });
      return {
        shortCircuited: true
      };
    }
    if (isDeferredResult(result)) {
      throw getInternalRouterError(400, {
        type: "defer-action"
      });
    }
    if (isErrorResult(result)) {
      // Store off the pending error - we use it to determine which loaders
      // to call and will commit it when we complete the navigation
      let boundaryMatch = findNearestBoundary(matches, actionMatch.route.id);
      // By default, all submissions to the current location are REPLACE
      // navigations, but if the action threw an error that'll be rendered in
      // an errorElement, we fall back to PUSH so that the user can use the
      // back button to get back to the pre-submission form location to try
      // again
      if ((opts && opts.replace) !== true) {
        pendingAction = Action.Push;
      }
      return {
        matches,
        pendingActionResult: [boundaryMatch.route.id, result]
      };
    }
    return {
      matches,
      pendingActionResult: [actionMatch.route.id, result]
    };
  }
  // Call all applicable loaders for the given matches, handling redirects,
  // errors, etc.
  async function handleLoaders(request, location, matches, isFogOfWar, overrideNavigation, submission, fetcherSubmission, replace, initialHydration, flushSync, pendingActionResult) {
    // Figure out the right navigation we want to use for data loading
    let loadingNavigation = overrideNavigation || getLoadingNavigation(location, submission);
    // If this was a redirect from an action we don't have a "submission" but
    // we have it on the loading navigation so use that if available
    let activeSubmission = submission || fetcherSubmission || getSubmissionFromNavigation(loadingNavigation);
    // If this is an uninterrupted revalidation, we remain in our current idle
    // state.  If not, we need to switch to our loading state and load data,
    // preserving any new action data or existing action data (in the case of
    // a revalidation interrupting an actionReload)
    // If we have partialHydration enabled, then don't update the state for the
    // initial data load since it's not a "navigation"
    let shouldUpdateNavigationState = !isUninterruptedRevalidation && (!future.v7_partialHydration || !initialHydration);
    // When fog of war is enabled, we enter our `loading` state earlier so we
    // can discover new routes during the `loading` state.  We skip this if
    // we've already run actions since we would have done our matching already.
    // If the children() function threw then, we want to proceed with the
    // partial matches it discovered.
    if (isFogOfWar) {
      if (shouldUpdateNavigationState) {
        let actionData = getUpdatedActionData(pendingActionResult);
        updateState(_extends$2({
          navigation: loadingNavigation
        }, actionData !== undefined ? {
          actionData
        } : {}), {
          flushSync
        });
      }
      let discoverResult = await discoverRoutes(matches, location.pathname, request.signal);
      if (discoverResult.type === "aborted") {
        return {
          shortCircuited: true
        };
      } else if (discoverResult.type === "error") {
        let boundaryId = findNearestBoundary(discoverResult.partialMatches).route.id;
        return {
          matches: discoverResult.partialMatches,
          loaderData: {},
          errors: {
            [boundaryId]: discoverResult.error
          }
        };
      } else if (!discoverResult.matches) {
        let {
          error,
          notFoundMatches,
          route
        } = handleNavigational404(location.pathname);
        return {
          matches: notFoundMatches,
          loaderData: {},
          errors: {
            [route.id]: error
          }
        };
      } else {
        matches = discoverResult.matches;
      }
    }
    let routesToUse = inFlightDataRoutes || dataRoutes;
    let [matchesToLoad, revalidatingFetchers] = getMatchesToLoad(init.history, state, matches, activeSubmission, location, future.v7_partialHydration && initialHydration === true, future.v7_skipActionErrorRevalidation, isRevalidationRequired, cancelledDeferredRoutes, cancelledFetcherLoads, deletedFetchers, fetchLoadMatches, fetchRedirectIds, routesToUse, basename, pendingActionResult);
    // Cancel pending deferreds for no-longer-matched routes or routes we're
    // about to reload.  Note that if this is an action reload we would have
    // already cancelled all pending deferreds so this would be a no-op
    cancelActiveDeferreds(routeId => !(matches && matches.some(m => m.route.id === routeId)) || matchesToLoad && matchesToLoad.some(m => m.route.id === routeId));
    pendingNavigationLoadId = ++incrementingLoadId;
    // Short circuit if we have no loaders to run
    if (matchesToLoad.length === 0 && revalidatingFetchers.length === 0) {
      let updatedFetchers = markFetchRedirectsDone();
      completeNavigation(location, _extends$2({
        matches,
        loaderData: {},
        // Commit pending error if we're short circuiting
        errors: pendingActionResult && isErrorResult(pendingActionResult[1]) ? {
          [pendingActionResult[0]]: pendingActionResult[1].error
        } : null
      }, getActionDataForCommit(pendingActionResult), updatedFetchers ? {
        fetchers: new Map(state.fetchers)
      } : {}), {
        flushSync
      });
      return {
        shortCircuited: true
      };
    }
    if (shouldUpdateNavigationState) {
      let updates = {};
      if (!isFogOfWar) {
        // Only update navigation/actionNData if we didn't already do it above
        updates.navigation = loadingNavigation;
        let actionData = getUpdatedActionData(pendingActionResult);
        if (actionData !== undefined) {
          updates.actionData = actionData;
        }
      }
      if (revalidatingFetchers.length > 0) {
        updates.fetchers = getUpdatedRevalidatingFetchers(revalidatingFetchers);
      }
      updateState(updates, {
        flushSync
      });
    }
    revalidatingFetchers.forEach(rf => {
      abortFetcher(rf.key);
      if (rf.controller) {
        // Fetchers use an independent AbortController so that aborting a fetcher
        // (via deleteFetcher) does not abort the triggering navigation that
        // triggered the revalidation
        fetchControllers.set(rf.key, rf.controller);
      }
    });
    // Proxy navigation abort through to revalidation fetchers
    let abortPendingFetchRevalidations = () => revalidatingFetchers.forEach(f => abortFetcher(f.key));
    if (pendingNavigationController) {
      pendingNavigationController.signal.addEventListener("abort", abortPendingFetchRevalidations);
    }
    let {
      loaderResults,
      fetcherResults
    } = await callLoadersAndMaybeResolveData(state, matches, matchesToLoad, revalidatingFetchers, request);
    if (request.signal.aborted) {
      return {
        shortCircuited: true
      };
    }
    // Clean up _after_ loaders have completed.  Don't clean up if we short
    // circuited because fetchControllers would have been aborted and
    // reassigned to new controllers for the next navigation
    if (pendingNavigationController) {
      pendingNavigationController.signal.removeEventListener("abort", abortPendingFetchRevalidations);
    }
    revalidatingFetchers.forEach(rf => fetchControllers.delete(rf.key));
    // If any loaders returned a redirect Response, start a new REPLACE navigation
    let redirect = findRedirect(loaderResults);
    if (redirect) {
      await startRedirectNavigation(request, redirect.result, true, {
        replace
      });
      return {
        shortCircuited: true
      };
    }
    redirect = findRedirect(fetcherResults);
    if (redirect) {
      // If this redirect came from a fetcher make sure we mark it in
      // fetchRedirectIds so it doesn't get revalidated on the next set of
      // loader executions
      fetchRedirectIds.add(redirect.key);
      await startRedirectNavigation(request, redirect.result, true, {
        replace
      });
      return {
        shortCircuited: true
      };
    }
    // Process and commit output from loaders
    let {
      loaderData,
      errors
    } = processLoaderData(state, matches, loaderResults, pendingActionResult, revalidatingFetchers, fetcherResults, activeDeferreds);
    // Wire up subscribers to update loaderData as promises settle
    activeDeferreds.forEach((deferredData, routeId) => {
      deferredData.subscribe(aborted => {
        // Note: No need to updateState here since the TrackedPromise on
        // loaderData is stable across resolve/reject
        // Remove this instance if we were aborted or if promises have settled
        if (aborted || deferredData.done) {
          activeDeferreds.delete(routeId);
        }
      });
    });
    // Preserve SSR errors during partial hydration
    if (future.v7_partialHydration && initialHydration && state.errors) {
      errors = _extends$2({}, state.errors, errors);
    }
    let updatedFetchers = markFetchRedirectsDone();
    let didAbortFetchLoads = abortStaleFetchLoads(pendingNavigationLoadId);
    let shouldUpdateFetchers = updatedFetchers || didAbortFetchLoads || revalidatingFetchers.length > 0;
    return _extends$2({
      matches,
      loaderData,
      errors
    }, shouldUpdateFetchers ? {
      fetchers: new Map(state.fetchers)
    } : {});
  }
  function getUpdatedActionData(pendingActionResult) {
    if (pendingActionResult && !isErrorResult(pendingActionResult[1])) {
      // This is cast to `any` currently because `RouteData`uses any and it
      // would be a breaking change to use any.
      // TODO: v7 - change `RouteData` to use `unknown` instead of `any`
      return {
        [pendingActionResult[0]]: pendingActionResult[1].data
      };
    } else if (state.actionData) {
      if (Object.keys(state.actionData).length === 0) {
        return null;
      } else {
        return state.actionData;
      }
    }
  }
  function getUpdatedRevalidatingFetchers(revalidatingFetchers) {
    revalidatingFetchers.forEach(rf => {
      let fetcher = state.fetchers.get(rf.key);
      let revalidatingFetcher = getLoadingFetcher(undefined, fetcher ? fetcher.data : undefined);
      state.fetchers.set(rf.key, revalidatingFetcher);
    });
    return new Map(state.fetchers);
  }
  // Trigger a fetcher load/submit for the given fetcher key
  function fetch(key, routeId, href, opts) {
    if (isServer) {
      throw new Error("router.fetch() was called during the server render, but it shouldn't be. " + "You are likely calling a useFetcher() method in the body of your component. " + "Try moving it to a useEffect or a callback.");
    }
    abortFetcher(key);
    let flushSync = (opts && opts.flushSync) === true;
    let routesToUse = inFlightDataRoutes || dataRoutes;
    let normalizedPath = normalizeTo(state.location, state.matches, basename, future.v7_prependBasename, href, future.v7_relativeSplatPath, routeId, opts == null ? void 0 : opts.relative);
    let matches = matchRoutes(routesToUse, normalizedPath, basename);
    let fogOfWar = checkFogOfWar(matches, routesToUse, normalizedPath);
    if (fogOfWar.active && fogOfWar.matches) {
      matches = fogOfWar.matches;
    }
    if (!matches) {
      setFetcherError(key, routeId, getInternalRouterError(404, {
        pathname: normalizedPath
      }), {
        flushSync
      });
      return;
    }
    let {
      path,
      submission,
      error
    } = normalizeNavigateOptions(future.v7_normalizeFormMethod, true, normalizedPath, opts);
    if (error) {
      setFetcherError(key, routeId, error, {
        flushSync
      });
      return;
    }
    let match = getTargetMatch(matches, path);
    let preventScrollReset = (opts && opts.preventScrollReset) === true;
    if (submission && isMutationMethod(submission.formMethod)) {
      handleFetcherAction(key, routeId, path, match, matches, fogOfWar.active, flushSync, preventScrollReset, submission);
      return;
    }
    // Store off the match so we can call it's shouldRevalidate on subsequent
    // revalidations
    fetchLoadMatches.set(key, {
      routeId,
      path
    });
    handleFetcherLoader(key, routeId, path, match, matches, fogOfWar.active, flushSync, preventScrollReset, submission);
  }
  // Call the action for the matched fetcher.submit(), and then handle redirects,
  // errors, and revalidation
  async function handleFetcherAction(key, routeId, path, match, requestMatches, isFogOfWar, flushSync, preventScrollReset, submission) {
    interruptActiveLoads();
    fetchLoadMatches.delete(key);
    function detectAndHandle405Error(m) {
      if (!m.route.action && !m.route.lazy) {
        let error = getInternalRouterError(405, {
          method: submission.formMethod,
          pathname: path,
          routeId: routeId
        });
        setFetcherError(key, routeId, error, {
          flushSync
        });
        return true;
      }
      return false;
    }
    if (!isFogOfWar && detectAndHandle405Error(match)) {
      return;
    }
    // Put this fetcher into it's submitting state
    let existingFetcher = state.fetchers.get(key);
    updateFetcherState(key, getSubmittingFetcher(submission, existingFetcher), {
      flushSync
    });
    let abortController = new AbortController();
    let fetchRequest = createClientSideRequest(init.history, path, abortController.signal, submission);
    if (isFogOfWar) {
      let discoverResult = await discoverRoutes(requestMatches, new URL(fetchRequest.url).pathname, fetchRequest.signal, key);
      if (discoverResult.type === "aborted") {
        return;
      } else if (discoverResult.type === "error") {
        setFetcherError(key, routeId, discoverResult.error, {
          flushSync
        });
        return;
      } else if (!discoverResult.matches) {
        setFetcherError(key, routeId, getInternalRouterError(404, {
          pathname: path
        }), {
          flushSync
        });
        return;
      } else {
        requestMatches = discoverResult.matches;
        match = getTargetMatch(requestMatches, path);
        if (detectAndHandle405Error(match)) {
          return;
        }
      }
    }
    // Call the action for the fetcher
    fetchControllers.set(key, abortController);
    let originatingLoadId = incrementingLoadId;
    let actionResults = await callDataStrategy("action", state, fetchRequest, [match], requestMatches, key);
    let actionResult = actionResults[match.route.id];
    if (fetchRequest.signal.aborted) {
      // We can delete this so long as we weren't aborted by our own fetcher
      // re-submit which would have put _new_ controller is in fetchControllers
      if (fetchControllers.get(key) === abortController) {
        fetchControllers.delete(key);
      }
      return;
    }
    // When using v7_fetcherPersist, we don't want errors bubbling up to the UI
    // or redirects processed for unmounted fetchers so we just revert them to
    // idle
    if (future.v7_fetcherPersist && deletedFetchers.has(key)) {
      if (isRedirectResult(actionResult) || isErrorResult(actionResult)) {
        updateFetcherState(key, getDoneFetcher(undefined));
        return;
      }
      // Let SuccessResult's fall through for revalidation
    } else {
      if (isRedirectResult(actionResult)) {
        fetchControllers.delete(key);
        if (pendingNavigationLoadId > originatingLoadId) {
          // A new navigation was kicked off after our action started, so that
          // should take precedence over this redirect navigation.  We already
          // set isRevalidationRequired so all loaders for the new route should
          // fire unless opted out via shouldRevalidate
          updateFetcherState(key, getDoneFetcher(undefined));
          return;
        } else {
          fetchRedirectIds.add(key);
          updateFetcherState(key, getLoadingFetcher(submission));
          return startRedirectNavigation(fetchRequest, actionResult, false, {
            fetcherSubmission: submission,
            preventScrollReset
          });
        }
      }
      // Process any non-redirect errors thrown
      if (isErrorResult(actionResult)) {
        setFetcherError(key, routeId, actionResult.error);
        return;
      }
    }
    if (isDeferredResult(actionResult)) {
      throw getInternalRouterError(400, {
        type: "defer-action"
      });
    }
    // Start the data load for current matches, or the next location if we're
    // in the middle of a navigation
    let nextLocation = state.navigation.location || state.location;
    let revalidationRequest = createClientSideRequest(init.history, nextLocation, abortController.signal);
    let routesToUse = inFlightDataRoutes || dataRoutes;
    let matches = state.navigation.state !== "idle" ? matchRoutes(routesToUse, state.navigation.location, basename) : state.matches;
    invariant(matches, "Didn't find any matches after fetcher action");
    let loadId = ++incrementingLoadId;
    fetchReloadIds.set(key, loadId);
    let loadFetcher = getLoadingFetcher(submission, actionResult.data);
    state.fetchers.set(key, loadFetcher);
    let [matchesToLoad, revalidatingFetchers] = getMatchesToLoad(init.history, state, matches, submission, nextLocation, false, future.v7_skipActionErrorRevalidation, isRevalidationRequired, cancelledDeferredRoutes, cancelledFetcherLoads, deletedFetchers, fetchLoadMatches, fetchRedirectIds, routesToUse, basename, [match.route.id, actionResult]);
    // Put all revalidating fetchers into the loading state, except for the
    // current fetcher which we want to keep in it's current loading state which
    // contains it's action submission info + action data
    revalidatingFetchers.filter(rf => rf.key !== key).forEach(rf => {
      let staleKey = rf.key;
      let existingFetcher = state.fetchers.get(staleKey);
      let revalidatingFetcher = getLoadingFetcher(undefined, existingFetcher ? existingFetcher.data : undefined);
      state.fetchers.set(staleKey, revalidatingFetcher);
      abortFetcher(staleKey);
      if (rf.controller) {
        fetchControllers.set(staleKey, rf.controller);
      }
    });
    updateState({
      fetchers: new Map(state.fetchers)
    });
    let abortPendingFetchRevalidations = () => revalidatingFetchers.forEach(rf => abortFetcher(rf.key));
    abortController.signal.addEventListener("abort", abortPendingFetchRevalidations);
    let {
      loaderResults,
      fetcherResults
    } = await callLoadersAndMaybeResolveData(state, matches, matchesToLoad, revalidatingFetchers, revalidationRequest);
    if (abortController.signal.aborted) {
      return;
    }
    abortController.signal.removeEventListener("abort", abortPendingFetchRevalidations);
    fetchReloadIds.delete(key);
    fetchControllers.delete(key);
    revalidatingFetchers.forEach(r => fetchControllers.delete(r.key));
    let redirect = findRedirect(loaderResults);
    if (redirect) {
      return startRedirectNavigation(revalidationRequest, redirect.result, false, {
        preventScrollReset
      });
    }
    redirect = findRedirect(fetcherResults);
    if (redirect) {
      // If this redirect came from a fetcher make sure we mark it in
      // fetchRedirectIds so it doesn't get revalidated on the next set of
      // loader executions
      fetchRedirectIds.add(redirect.key);
      return startRedirectNavigation(revalidationRequest, redirect.result, false, {
        preventScrollReset
      });
    }
    // Process and commit output from loaders
    let {
      loaderData,
      errors
    } = processLoaderData(state, matches, loaderResults, undefined, revalidatingFetchers, fetcherResults, activeDeferreds);
    // Since we let revalidations complete even if the submitting fetcher was
    // deleted, only put it back to idle if it hasn't been deleted
    if (state.fetchers.has(key)) {
      let doneFetcher = getDoneFetcher(actionResult.data);
      state.fetchers.set(key, doneFetcher);
    }
    abortStaleFetchLoads(loadId);
    // If we are currently in a navigation loading state and this fetcher is
    // more recent than the navigation, we want the newer data so abort the
    // navigation and complete it with the fetcher data
    if (state.navigation.state === "loading" && loadId > pendingNavigationLoadId) {
      invariant(pendingAction, "Expected pending action");
      pendingNavigationController && pendingNavigationController.abort();
      completeNavigation(state.navigation.location, {
        matches,
        loaderData,
        errors,
        fetchers: new Map(state.fetchers)
      });
    } else {
      // otherwise just update with the fetcher data, preserving any existing
      // loaderData for loaders that did not need to reload.  We have to
      // manually merge here since we aren't going through completeNavigation
      updateState({
        errors,
        loaderData: mergeLoaderData(state.loaderData, loaderData, matches, errors),
        fetchers: new Map(state.fetchers)
      });
      isRevalidationRequired = false;
    }
  }
  // Call the matched loader for fetcher.load(), handling redirects, errors, etc.
  async function handleFetcherLoader(key, routeId, path, match, matches, isFogOfWar, flushSync, preventScrollReset, submission) {
    let existingFetcher = state.fetchers.get(key);
    updateFetcherState(key, getLoadingFetcher(submission, existingFetcher ? existingFetcher.data : undefined), {
      flushSync
    });
    let abortController = new AbortController();
    let fetchRequest = createClientSideRequest(init.history, path, abortController.signal);
    if (isFogOfWar) {
      let discoverResult = await discoverRoutes(matches, new URL(fetchRequest.url).pathname, fetchRequest.signal, key);
      if (discoverResult.type === "aborted") {
        return;
      } else if (discoverResult.type === "error") {
        setFetcherError(key, routeId, discoverResult.error, {
          flushSync
        });
        return;
      } else if (!discoverResult.matches) {
        setFetcherError(key, routeId, getInternalRouterError(404, {
          pathname: path
        }), {
          flushSync
        });
        return;
      } else {
        matches = discoverResult.matches;
        match = getTargetMatch(matches, path);
      }
    }
    // Call the loader for this fetcher route match
    fetchControllers.set(key, abortController);
    let originatingLoadId = incrementingLoadId;
    let results = await callDataStrategy("loader", state, fetchRequest, [match], matches, key);
    let result = results[match.route.id];
    // Deferred isn't supported for fetcher loads, await everything and treat it
    // as a normal load.  resolveDeferredData will return undefined if this
    // fetcher gets aborted, so we just leave result untouched and short circuit
    // below if that happens
    if (isDeferredResult(result)) {
      result = (await resolveDeferredData(result, fetchRequest.signal, true)) || result;
    }
    // We can delete this so long as we weren't aborted by our our own fetcher
    // re-load which would have put _new_ controller is in fetchControllers
    if (fetchControllers.get(key) === abortController) {
      fetchControllers.delete(key);
    }
    if (fetchRequest.signal.aborted) {
      return;
    }
    // We don't want errors bubbling up or redirects followed for unmounted
    // fetchers, so short circuit here if it was removed from the UI
    if (deletedFetchers.has(key)) {
      updateFetcherState(key, getDoneFetcher(undefined));
      return;
    }
    // If the loader threw a redirect Response, start a new REPLACE navigation
    if (isRedirectResult(result)) {
      if (pendingNavigationLoadId > originatingLoadId) {
        // A new navigation was kicked off after our loader started, so that
        // should take precedence over this redirect navigation
        updateFetcherState(key, getDoneFetcher(undefined));
        return;
      } else {
        fetchRedirectIds.add(key);
        await startRedirectNavigation(fetchRequest, result, false, {
          preventScrollReset
        });
        return;
      }
    }
    // Process any non-redirect errors thrown
    if (isErrorResult(result)) {
      setFetcherError(key, routeId, result.error);
      return;
    }
    invariant(!isDeferredResult(result), "Unhandled fetcher deferred data");
    // Put the fetcher back into an idle state
    updateFetcherState(key, getDoneFetcher(result.data));
  }
  /**
   * Utility function to handle redirects returned from an action or loader.
   * Normally, a redirect "replaces" the navigation that triggered it.  So, for
   * example:
   *
   *  - user is on /a
   *  - user clicks a link to /b
   *  - loader for /b redirects to /c
   *
   * In a non-JS app the browser would track the in-flight navigation to /b and
   * then replace it with /c when it encountered the redirect response.  In
   * the end it would only ever update the URL bar with /c.
   *
   * In client-side routing using pushState/replaceState, we aim to emulate
   * this behavior and we also do not update history until the end of the
   * navigation (including processed redirects).  This means that we never
   * actually touch history until we've processed redirects, so we just use
   * the history action from the original navigation (PUSH or REPLACE).
   */
  async function startRedirectNavigation(request, redirect, isNavigation, _temp2) {
    let {
      submission,
      fetcherSubmission,
      preventScrollReset,
      replace
    } = _temp2 === void 0 ? {} : _temp2;
    if (redirect.response.headers.has("X-Remix-Revalidate")) {
      isRevalidationRequired = true;
    }
    let location = redirect.response.headers.get("Location");
    invariant(location, "Expected a Location header on the redirect Response");
    location = normalizeRedirectLocation(location, new URL(request.url), basename);
    let redirectLocation = createLocation(state.location, location, {
      _isRedirect: true
    });
    if (isBrowser) {
      let isDocumentReload = false;
      if (redirect.response.headers.has("X-Remix-Reload-Document")) {
        // Hard reload if the response contained X-Remix-Reload-Document
        isDocumentReload = true;
      } else if (ABSOLUTE_URL_REGEX$2.test(location)) {
        const url = init.history.createURL(location);
        isDocumentReload =
        // Hard reload if it's an absolute URL to a new origin
        url.origin !== routerWindow.location.origin ||
        // Hard reload if it's an absolute URL that does not match our basename
        stripBasename(url.pathname, basename) == null;
      }
      if (isDocumentReload) {
        if (replace) {
          routerWindow.location.replace(location);
        } else {
          routerWindow.location.assign(location);
        }
        return;
      }
    }
    // There's no need to abort on redirects, since we don't detect the
    // redirect until the action/loaders have settled
    pendingNavigationController = null;
    let redirectHistoryAction = replace === true || redirect.response.headers.has("X-Remix-Replace") ? Action.Replace : Action.Push;
    // Use the incoming submission if provided, fallback on the active one in
    // state.navigation
    let {
      formMethod,
      formAction,
      formEncType
    } = state.navigation;
    if (!submission && !fetcherSubmission && formMethod && formAction && formEncType) {
      submission = getSubmissionFromNavigation(state.navigation);
    }
    // If this was a 307/308 submission we want to preserve the HTTP method and
    // re-submit the GET/POST/PUT/PATCH/DELETE as a submission navigation to the
    // redirected location
    let activeSubmission = submission || fetcherSubmission;
    if (redirectPreserveMethodStatusCodes.has(redirect.response.status) && activeSubmission && isMutationMethod(activeSubmission.formMethod)) {
      await startNavigation(redirectHistoryAction, redirectLocation, {
        submission: _extends$2({}, activeSubmission, {
          formAction: location
        }),
        // Preserve these flags across redirects
        preventScrollReset: preventScrollReset || pendingPreventScrollReset,
        enableViewTransition: isNavigation ? pendingViewTransitionEnabled : undefined
      });
    } else {
      // If we have a navigation submission, we will preserve it through the
      // redirect navigation
      let overrideNavigation = getLoadingNavigation(redirectLocation, submission);
      await startNavigation(redirectHistoryAction, redirectLocation, {
        overrideNavigation,
        // Send fetcher submissions through for shouldRevalidate
        fetcherSubmission,
        // Preserve these flags across redirects
        preventScrollReset: preventScrollReset || pendingPreventScrollReset,
        enableViewTransition: isNavigation ? pendingViewTransitionEnabled : undefined
      });
    }
  }
  // Utility wrapper for calling dataStrategy client-side without having to
  // pass around the manifest, mapRouteProperties, etc.
  async function callDataStrategy(type, state, request, matchesToLoad, matches, fetcherKey) {
    let results;
    let dataResults = {};
    try {
      results = await callDataStrategyImpl(dataStrategyImpl, type, state, request, matchesToLoad, matches, fetcherKey, manifest, mapRouteProperties);
    } catch (e) {
      // If the outer dataStrategy method throws, just return the error for all
      // matches - and it'll naturally bubble to the root
      matchesToLoad.forEach(m => {
        dataResults[m.route.id] = {
          type: ResultType.error,
          error: e
        };
      });
      return dataResults;
    }
    for (let [routeId, result] of Object.entries(results)) {
      if (isRedirectDataStrategyResultResult(result)) {
        let response = result.result;
        dataResults[routeId] = {
          type: ResultType.redirect,
          response: normalizeRelativeRoutingRedirectResponse(response, request, routeId, matches, basename, future.v7_relativeSplatPath)
        };
      } else {
        dataResults[routeId] = await convertDataStrategyResultToDataResult(result);
      }
    }
    return dataResults;
  }
  async function callLoadersAndMaybeResolveData(state, matches, matchesToLoad, fetchersToLoad, request) {
    let currentMatches = state.matches;
    // Kick off loaders and fetchers in parallel
    let loaderResultsPromise = callDataStrategy("loader", state, request, matchesToLoad, matches, null);
    let fetcherResultsPromise = Promise.all(fetchersToLoad.map(async f => {
      if (f.matches && f.match && f.controller) {
        let results = await callDataStrategy("loader", state, createClientSideRequest(init.history, f.path, f.controller.signal), [f.match], f.matches, f.key);
        let result = results[f.match.route.id];
        // Fetcher results are keyed by fetcher key from here on out, not routeId
        return {
          [f.key]: result
        };
      } else {
        return Promise.resolve({
          [f.key]: {
            type: ResultType.error,
            error: getInternalRouterError(404, {
              pathname: f.path
            })
          }
        });
      }
    }));
    let loaderResults = await loaderResultsPromise;
    let fetcherResults = (await fetcherResultsPromise).reduce((acc, r) => Object.assign(acc, r), {});
    await Promise.all([resolveNavigationDeferredResults(matches, loaderResults, request.signal, currentMatches, state.loaderData), resolveFetcherDeferredResults(matches, fetcherResults, fetchersToLoad)]);
    return {
      loaderResults,
      fetcherResults
    };
  }
  function interruptActiveLoads() {
    // Every interruption triggers a revalidation
    isRevalidationRequired = true;
    // Cancel pending route-level deferreds and mark cancelled routes for
    // revalidation
    cancelledDeferredRoutes.push(...cancelActiveDeferreds());
    // Abort in-flight fetcher loads
    fetchLoadMatches.forEach((_, key) => {
      if (fetchControllers.has(key)) {
        cancelledFetcherLoads.add(key);
      }
      abortFetcher(key);
    });
  }
  function updateFetcherState(key, fetcher, opts) {
    if (opts === void 0) {
      opts = {};
    }
    state.fetchers.set(key, fetcher);
    updateState({
      fetchers: new Map(state.fetchers)
    }, {
      flushSync: (opts && opts.flushSync) === true
    });
  }
  function setFetcherError(key, routeId, error, opts) {
    if (opts === void 0) {
      opts = {};
    }
    let boundaryMatch = findNearestBoundary(state.matches, routeId);
    deleteFetcher(key);
    updateState({
      errors: {
        [boundaryMatch.route.id]: error
      },
      fetchers: new Map(state.fetchers)
    }, {
      flushSync: (opts && opts.flushSync) === true
    });
  }
  function getFetcher(key) {
    activeFetchers.set(key, (activeFetchers.get(key) || 0) + 1);
    // If this fetcher was previously marked for deletion, unmark it since we
    // have a new instance
    if (deletedFetchers.has(key)) {
      deletedFetchers.delete(key);
    }
    return state.fetchers.get(key) || IDLE_FETCHER;
  }
  function deleteFetcher(key) {
    let fetcher = state.fetchers.get(key);
    // Don't abort the controller if this is a deletion of a fetcher.submit()
    // in it's loading phase since - we don't want to abort the corresponding
    // revalidation and want them to complete and land
    if (fetchControllers.has(key) && !(fetcher && fetcher.state === "loading" && fetchReloadIds.has(key))) {
      abortFetcher(key);
    }
    fetchLoadMatches.delete(key);
    fetchReloadIds.delete(key);
    fetchRedirectIds.delete(key);
    // If we opted into the flag we can clear this now since we're calling
    // deleteFetcher() at the end of updateState() and we've already handed the
    // deleted fetcher keys off to the data layer.
    // If not, we're eagerly calling deleteFetcher() and we need to keep this
    // Set populated until the next updateState call, and we'll clear
    // `deletedFetchers` then
    if (future.v7_fetcherPersist) {
      deletedFetchers.delete(key);
    }
    cancelledFetcherLoads.delete(key);
    state.fetchers.delete(key);
  }
  function deleteFetcherAndUpdateState(key) {
    let count = (activeFetchers.get(key) || 0) - 1;
    if (count <= 0) {
      activeFetchers.delete(key);
      deletedFetchers.add(key);
      if (!future.v7_fetcherPersist) {
        deleteFetcher(key);
      }
    } else {
      activeFetchers.set(key, count);
    }
    updateState({
      fetchers: new Map(state.fetchers)
    });
  }
  function abortFetcher(key) {
    let controller = fetchControllers.get(key);
    if (controller) {
      controller.abort();
      fetchControllers.delete(key);
    }
  }
  function markFetchersDone(keys) {
    for (let key of keys) {
      let fetcher = getFetcher(key);
      let doneFetcher = getDoneFetcher(fetcher.data);
      state.fetchers.set(key, doneFetcher);
    }
  }
  function markFetchRedirectsDone() {
    let doneKeys = [];
    let updatedFetchers = false;
    for (let key of fetchRedirectIds) {
      let fetcher = state.fetchers.get(key);
      invariant(fetcher, "Expected fetcher: " + key);
      if (fetcher.state === "loading") {
        fetchRedirectIds.delete(key);
        doneKeys.push(key);
        updatedFetchers = true;
      }
    }
    markFetchersDone(doneKeys);
    return updatedFetchers;
  }
  function abortStaleFetchLoads(landedId) {
    let yeetedKeys = [];
    for (let [key, id] of fetchReloadIds) {
      if (id < landedId) {
        let fetcher = state.fetchers.get(key);
        invariant(fetcher, "Expected fetcher: " + key);
        if (fetcher.state === "loading") {
          abortFetcher(key);
          fetchReloadIds.delete(key);
          yeetedKeys.push(key);
        }
      }
    }
    markFetchersDone(yeetedKeys);
    return yeetedKeys.length > 0;
  }
  function getBlocker(key, fn) {
    let blocker = state.blockers.get(key) || IDLE_BLOCKER;
    if (blockerFunctions.get(key) !== fn) {
      blockerFunctions.set(key, fn);
    }
    return blocker;
  }
  function deleteBlocker(key) {
    state.blockers.delete(key);
    blockerFunctions.delete(key);
  }
  // Utility function to update blockers, ensuring valid state transitions
  function updateBlocker(key, newBlocker) {
    let blocker = state.blockers.get(key) || IDLE_BLOCKER;
    // Poor mans state machine :)
    // https://mermaid.live/edit#pako:eNqVkc9OwzAMxl8l8nnjAYrEtDIOHEBIgwvKJTReGy3_lDpIqO27k6awMG0XcrLlnz87nwdonESogKXXBuE79rq75XZO3-yHds0RJVuv70YrPlUrCEe2HfrORS3rubqZfuhtpg5C9wk5tZ4VKcRUq88q9Z8RS0-48cE1iHJkL0ugbHuFLus9L6spZy8nX9MP2CNdomVaposqu3fGayT8T8-jJQwhepo_UtpgBQaDEUom04dZhAN1aJBDlUKJBxE1ceB2Smj0Mln-IBW5AFU2dwUiktt_2Qaq2dBfaKdEup85UV7Yd-dKjlnkabl2Pvr0DTkTreM
    invariant(blocker.state === "unblocked" && newBlocker.state === "blocked" || blocker.state === "blocked" && newBlocker.state === "blocked" || blocker.state === "blocked" && newBlocker.state === "proceeding" || blocker.state === "blocked" && newBlocker.state === "unblocked" || blocker.state === "proceeding" && newBlocker.state === "unblocked", "Invalid blocker state transition: " + blocker.state + " -> " + newBlocker.state);
    let blockers = new Map(state.blockers);
    blockers.set(key, newBlocker);
    updateState({
      blockers
    });
  }
  function shouldBlockNavigation(_ref2) {
    let {
      currentLocation,
      nextLocation,
      historyAction
    } = _ref2;
    if (blockerFunctions.size === 0) {
      return;
    }
    // We ony support a single active blocker at the moment since we don't have
    // any compelling use cases for multi-blocker yet
    if (blockerFunctions.size > 1) {
      warning(false, "A router only supports one blocker at a time");
    }
    let entries = Array.from(blockerFunctions.entries());
    let [blockerKey, blockerFunction] = entries[entries.length - 1];
    let blocker = state.blockers.get(blockerKey);
    if (blocker && blocker.state === "proceeding") {
      // If the blocker is currently proceeding, we don't need to re-check
      // it and can let this navigation continue
      return;
    }
    // At this point, we know we're unblocked/blocked so we need to check the
    // user-provided blocker function
    if (blockerFunction({
      currentLocation,
      nextLocation,
      historyAction
    })) {
      return blockerKey;
    }
  }
  function handleNavigational404(pathname) {
    let error = getInternalRouterError(404, {
      pathname
    });
    let routesToUse = inFlightDataRoutes || dataRoutes;
    let {
      matches,
      route
    } = getShortCircuitMatches(routesToUse);
    // Cancel all pending deferred on 404s since we don't keep any routes
    cancelActiveDeferreds();
    return {
      notFoundMatches: matches,
      route,
      error
    };
  }
  function cancelActiveDeferreds(predicate) {
    let cancelledRouteIds = [];
    activeDeferreds.forEach((dfd, routeId) => {
      if (!predicate || predicate(routeId)) {
        // Cancel the deferred - but do not remove from activeDeferreds here -
        // we rely on the subscribers to do that so our tests can assert proper
        // cleanup via _internalActiveDeferreds
        dfd.cancel();
        cancelledRouteIds.push(routeId);
        activeDeferreds.delete(routeId);
      }
    });
    return cancelledRouteIds;
  }
  // Opt in to capturing and reporting scroll positions during navigations,
  // used by the <ScrollRestoration> component
  function enableScrollRestoration(positions, getPosition, getKey) {
    savedScrollPositions = positions;
    getScrollPosition = getPosition;
    getScrollRestorationKey = getKey || null;
    // Perform initial hydration scroll restoration, since we miss the boat on
    // the initial updateState() because we've not yet rendered <ScrollRestoration/>
    // and therefore have no savedScrollPositions available
    if (!initialScrollRestored && state.navigation === IDLE_NAVIGATION) {
      initialScrollRestored = true;
      let y = getSavedScrollPosition(state.location, state.matches);
      if (y != null) {
        updateState({
          restoreScrollPosition: y
        });
      }
    }
    return () => {
      savedScrollPositions = null;
      getScrollPosition = null;
      getScrollRestorationKey = null;
    };
  }
  function getScrollKey(location, matches) {
    if (getScrollRestorationKey) {
      let key = getScrollRestorationKey(location, matches.map(m => convertRouteMatchToUiMatch(m, state.loaderData)));
      return key || location.key;
    }
    return location.key;
  }
  function saveScrollPosition(location, matches) {
    if (savedScrollPositions && getScrollPosition) {
      let key = getScrollKey(location, matches);
      savedScrollPositions[key] = getScrollPosition();
    }
  }
  function getSavedScrollPosition(location, matches) {
    if (savedScrollPositions) {
      let key = getScrollKey(location, matches);
      let y = savedScrollPositions[key];
      if (typeof y === "number") {
        return y;
      }
    }
    return null;
  }
  function checkFogOfWar(matches, routesToUse, pathname) {
    if (patchRoutesOnNavigationImpl) {
      if (!matches) {
        let fogMatches = matchRoutesImpl(routesToUse, pathname, basename, true);
        return {
          active: true,
          matches: fogMatches || []
        };
      } else {
        if (Object.keys(matches[0].params).length > 0) {
          // If we matched a dynamic param or a splat, it might only be because
          // we haven't yet discovered other routes that would match with a
          // higher score.  Call patchRoutesOnNavigation just to be sure
          let partialMatches = matchRoutesImpl(routesToUse, pathname, basename, true);
          return {
            active: true,
            matches: partialMatches
          };
        }
      }
    }
    return {
      active: false,
      matches: null
    };
  }
  async function discoverRoutes(matches, pathname, signal, fetcherKey) {
    if (!patchRoutesOnNavigationImpl) {
      return {
        type: "success",
        matches
      };
    }
    let partialMatches = matches;
    while (true) {
      let isNonHMR = inFlightDataRoutes == null;
      let routesToUse = inFlightDataRoutes || dataRoutes;
      let localManifest = manifest;
      try {
        await patchRoutesOnNavigationImpl({
          signal,
          path: pathname,
          matches: partialMatches,
          fetcherKey,
          patch: (routeId, children) => {
            if (signal.aborted) return;
            patchRoutesImpl(routeId, children, routesToUse, localManifest, mapRouteProperties);
          }
        });
      } catch (e) {
        return {
          type: "error",
          error: e,
          partialMatches
        };
      } finally {
        // If we are not in the middle of an HMR revalidation and we changed the
        // routes, provide a new identity so when we `updateState` at the end of
        // this navigation/fetch `router.routes` will be a new identity and
        // trigger a re-run of memoized `router.routes` dependencies.
        // HMR will already update the identity and reflow when it lands
        // `inFlightDataRoutes` in `completeNavigation`
        if (isNonHMR && !signal.aborted) {
          dataRoutes = [...dataRoutes];
        }
      }
      if (signal.aborted) {
        return {
          type: "aborted"
        };
      }
      let newMatches = matchRoutes(routesToUse, pathname, basename);
      if (newMatches) {
        return {
          type: "success",
          matches: newMatches
        };
      }
      let newPartialMatches = matchRoutesImpl(routesToUse, pathname, basename, true);
      // Avoid loops if the second pass results in the same partial matches
      if (!newPartialMatches || partialMatches.length === newPartialMatches.length && partialMatches.every((m, i) => m.route.id === newPartialMatches[i].route.id)) {
        return {
          type: "success",
          matches: null
        };
      }
      partialMatches = newPartialMatches;
    }
  }
  function _internalSetRoutes(newRoutes) {
    manifest = {};
    inFlightDataRoutes = convertRoutesToDataRoutes(newRoutes, mapRouteProperties, undefined, manifest);
  }
  function patchRoutes(routeId, children) {
    let isNonHMR = inFlightDataRoutes == null;
    let routesToUse = inFlightDataRoutes || dataRoutes;
    patchRoutesImpl(routeId, children, routesToUse, manifest, mapRouteProperties);
    // If we are not in the middle of an HMR revalidation and we changed the
    // routes, provide a new identity and trigger a reflow via `updateState`
    // to re-run memoized `router.routes` dependencies.
    // HMR will already update the identity and reflow when it lands
    // `inFlightDataRoutes` in `completeNavigation`
    if (isNonHMR) {
      dataRoutes = [...dataRoutes];
      updateState({});
    }
  }
  router = {
    get basename() {
      return basename;
    },
    get future() {
      return future;
    },
    get state() {
      return state;
    },
    get routes() {
      return dataRoutes;
    },
    get window() {
      return routerWindow;
    },
    initialize,
    subscribe,
    enableScrollRestoration,
    navigate,
    fetch,
    revalidate,
    // Passthrough to history-aware createHref used by useHref so we get proper
    // hash-aware URLs in DOM paths
    createHref: to => init.history.createHref(to),
    encodeLocation: to => init.history.encodeLocation(to),
    getFetcher,
    deleteFetcher: deleteFetcherAndUpdateState,
    dispose,
    getBlocker,
    deleteBlocker,
    patchRoutes,
    _internalFetchControllers: fetchControllers,
    _internalActiveDeferreds: activeDeferreds,
    // TODO: Remove setRoutes, it's temporary to avoid dealing with
    // updating the tree while validating the update algorithm.
    _internalSetRoutes
  };
  return router;
}
//#endregion
////////////////////////////////////////////////////////////////////////////////
//#region createStaticHandler
////////////////////////////////////////////////////////////////////////////////
const UNSAFE_DEFERRED_SYMBOL = Symbol("deferred");
function createStaticHandler(routes, opts) {
  invariant(routes.length > 0, "You must provide a non-empty routes array to createStaticHandler");
  let manifest = {};
  let basename = (opts ? opts.basename : null) || "/";
  let mapRouteProperties;
  if (opts != null && opts.mapRouteProperties) {
    mapRouteProperties = opts.mapRouteProperties;
  } else if (opts != null && opts.detectErrorBoundary) {
    // If they are still using the deprecated version, wrap it with the new API
    let detectErrorBoundary = opts.detectErrorBoundary;
    mapRouteProperties = route => ({
      hasErrorBoundary: detectErrorBoundary(route)
    });
  } else {
    mapRouteProperties = defaultMapRouteProperties;
  }
  // Config driven behavior flags
  let future = _extends$2({
    v7_relativeSplatPath: false,
    v7_throwAbortReason: false
  }, opts ? opts.future : null);
  let dataRoutes = convertRoutesToDataRoutes(routes, mapRouteProperties, undefined, manifest);
  /**
   * The query() method is intended for document requests, in which we want to
   * call an optional action and potentially multiple loaders for all nested
   * routes.  It returns a StaticHandlerContext object, which is very similar
   * to the router state (location, loaderData, actionData, errors, etc.) and
   * also adds SSR-specific information such as the statusCode and headers
   * from action/loaders Responses.
   *
   * It _should_ never throw and should report all errors through the
   * returned context.errors object, properly associating errors to their error
   * boundary.  Additionally, it tracks _deepestRenderedBoundaryId which can be
   * used to emulate React error boundaries during SSr by performing a second
   * pass only down to the boundaryId.
   *
   * The one exception where we do not return a StaticHandlerContext is when a
   * redirect response is returned or thrown from any action/loader.  We
   * propagate that out and return the raw Response so the HTTP server can
   * return it directly.
   *
   * - `opts.requestContext` is an optional server context that will be passed
   *   to actions/loaders in the `context` parameter
   * - `opts.skipLoaderErrorBubbling` is an optional parameter that will prevent
   *   the bubbling of errors which allows single-fetch-type implementations
   *   where the client will handle the bubbling and we may need to return data
   *   for the handling route
   */
  async function query(request, _temp3) {
    let {
      requestContext,
      skipLoaderErrorBubbling,
      dataStrategy
    } = _temp3 === void 0 ? {} : _temp3;
    let url = new URL(request.url);
    let method = request.method;
    let location = createLocation("", createPath(url), null, "default");
    let matches = matchRoutes(dataRoutes, location, basename);
    // SSR supports HEAD requests while SPA doesn't
    if (!isValidMethod(method) && method !== "HEAD") {
      let error = getInternalRouterError(405, {
        method
      });
      let {
        matches: methodNotAllowedMatches,
        route
      } = getShortCircuitMatches(dataRoutes);
      return {
        basename,
        location,
        matches: methodNotAllowedMatches,
        loaderData: {},
        actionData: null,
        errors: {
          [route.id]: error
        },
        statusCode: error.status,
        loaderHeaders: {},
        actionHeaders: {},
        activeDeferreds: null
      };
    } else if (!matches) {
      let error = getInternalRouterError(404, {
        pathname: location.pathname
      });
      let {
        matches: notFoundMatches,
        route
      } = getShortCircuitMatches(dataRoutes);
      return {
        basename,
        location,
        matches: notFoundMatches,
        loaderData: {},
        actionData: null,
        errors: {
          [route.id]: error
        },
        statusCode: error.status,
        loaderHeaders: {},
        actionHeaders: {},
        activeDeferreds: null
      };
    }
    let result = await queryImpl(request, location, matches, requestContext, dataStrategy || null, skipLoaderErrorBubbling === true, null);
    if (isResponse(result)) {
      return result;
    }
    // When returning StaticHandlerContext, we patch back in the location here
    // since we need it for React Context.  But this helps keep our submit and
    // loadRouteData operating on a Request instead of a Location
    return _extends$2({
      location,
      basename
    }, result);
  }
  /**
   * The queryRoute() method is intended for targeted route requests, either
   * for fetch ?_data requests or resource route requests.  In this case, we
   * are only ever calling a single action or loader, and we are returning the
   * returned value directly.  In most cases, this will be a Response returned
   * from the action/loader, but it may be a primitive or other value as well -
   * and in such cases the calling context should handle that accordingly.
   *
   * We do respect the throw/return differentiation, so if an action/loader
   * throws, then this method will throw the value.  This is important so we
   * can do proper boundary identification in Remix where a thrown Response
   * must go to the Catch Boundary but a returned Response is happy-path.
   *
   * One thing to note is that any Router-initiated Errors that make sense
   * to associate with a status code will be thrown as an ErrorResponse
   * instance which include the raw Error, such that the calling context can
   * serialize the error as they see fit while including the proper response
   * code.  Examples here are 404 and 405 errors that occur prior to reaching
   * any user-defined loaders.
   *
   * - `opts.routeId` allows you to specify the specific route handler to call.
   *   If not provided the handler will determine the proper route by matching
   *   against `request.url`
   * - `opts.requestContext` is an optional server context that will be passed
   *    to actions/loaders in the `context` parameter
   */
  async function queryRoute(request, _temp4) {
    let {
      routeId,
      requestContext,
      dataStrategy
    } = _temp4 === void 0 ? {} : _temp4;
    let url = new URL(request.url);
    let method = request.method;
    let location = createLocation("", createPath(url), null, "default");
    let matches = matchRoutes(dataRoutes, location, basename);
    // SSR supports HEAD requests while SPA doesn't
    if (!isValidMethod(method) && method !== "HEAD" && method !== "OPTIONS") {
      throw getInternalRouterError(405, {
        method
      });
    } else if (!matches) {
      throw getInternalRouterError(404, {
        pathname: location.pathname
      });
    }
    let match = routeId ? matches.find(m => m.route.id === routeId) : getTargetMatch(matches, location);
    if (routeId && !match) {
      throw getInternalRouterError(403, {
        pathname: location.pathname,
        routeId
      });
    } else if (!match) {
      // This should never hit I don't think?
      throw getInternalRouterError(404, {
        pathname: location.pathname
      });
    }
    let result = await queryImpl(request, location, matches, requestContext, dataStrategy || null, false, match);
    if (isResponse(result)) {
      return result;
    }
    let error = result.errors ? Object.values(result.errors)[0] : undefined;
    if (error !== undefined) {
      // If we got back result.errors, that means the loader/action threw
      // _something_ that wasn't a Response, but it's not guaranteed/required
      // to be an `instanceof Error` either, so we have to use throw here to
      // preserve the "error" state outside of queryImpl.
      throw error;
    }
    // Pick off the right state value to return
    if (result.actionData) {
      return Object.values(result.actionData)[0];
    }
    if (result.loaderData) {
      var _result$activeDeferre;
      let data = Object.values(result.loaderData)[0];
      if ((_result$activeDeferre = result.activeDeferreds) != null && _result$activeDeferre[match.route.id]) {
        data[UNSAFE_DEFERRED_SYMBOL] = result.activeDeferreds[match.route.id];
      }
      return data;
    }
    return undefined;
  }
  async function queryImpl(request, location, matches, requestContext, dataStrategy, skipLoaderErrorBubbling, routeMatch) {
    invariant(request.signal, "query()/queryRoute() requests must contain an AbortController signal");
    try {
      if (isMutationMethod(request.method.toLowerCase())) {
        let result = await submit(request, matches, routeMatch || getTargetMatch(matches, location), requestContext, dataStrategy, skipLoaderErrorBubbling, routeMatch != null);
        return result;
      }
      let result = await loadRouteData(request, matches, requestContext, dataStrategy, skipLoaderErrorBubbling, routeMatch);
      return isResponse(result) ? result : _extends$2({}, result, {
        actionData: null,
        actionHeaders: {}
      });
    } catch (e) {
      // If the user threw/returned a Response in callLoaderOrAction for a
      // `queryRoute` call, we throw the `DataStrategyResult` to bail out early
      // and then return or throw the raw Response here accordingly
      if (isDataStrategyResult(e) && isResponse(e.result)) {
        if (e.type === ResultType.error) {
          throw e.result;
        }
        return e.result;
      }
      // Redirects are always returned since they don't propagate to catch
      // boundaries
      if (isRedirectResponse(e)) {
        return e;
      }
      throw e;
    }
  }
  async function submit(request, matches, actionMatch, requestContext, dataStrategy, skipLoaderErrorBubbling, isRouteRequest) {
    let result;
    if (!actionMatch.route.action && !actionMatch.route.lazy) {
      let error = getInternalRouterError(405, {
        method: request.method,
        pathname: new URL(request.url).pathname,
        routeId: actionMatch.route.id
      });
      if (isRouteRequest) {
        throw error;
      }
      result = {
        type: ResultType.error,
        error
      };
    } else {
      let results = await callDataStrategy("action", request, [actionMatch], matches, isRouteRequest, requestContext, dataStrategy);
      result = results[actionMatch.route.id];
      if (request.signal.aborted) {
        throwStaticHandlerAbortedError(request, isRouteRequest, future);
      }
    }
    if (isRedirectResult(result)) {
      // Uhhhh - this should never happen, we should always throw these from
      // callLoaderOrAction, but the type narrowing here keeps TS happy and we
      // can get back on the "throw all redirect responses" train here should
      // this ever happen :/
      throw new Response(null, {
        status: result.response.status,
        headers: {
          Location: result.response.headers.get("Location")
        }
      });
    }
    if (isDeferredResult(result)) {
      let error = getInternalRouterError(400, {
        type: "defer-action"
      });
      if (isRouteRequest) {
        throw error;
      }
      result = {
        type: ResultType.error,
        error
      };
    }
    if (isRouteRequest) {
      // Note: This should only be non-Response values if we get here, since
      // isRouteRequest should throw any Response received in callLoaderOrAction
      if (isErrorResult(result)) {
        throw result.error;
      }
      return {
        matches: [actionMatch],
        loaderData: {},
        actionData: {
          [actionMatch.route.id]: result.data
        },
        errors: null,
        // Note: statusCode + headers are unused here since queryRoute will
        // return the raw Response or value
        statusCode: 200,
        loaderHeaders: {},
        actionHeaders: {},
        activeDeferreds: null
      };
    }
    // Create a GET request for the loaders
    let loaderRequest = new Request(request.url, {
      headers: request.headers,
      redirect: request.redirect,
      signal: request.signal
    });
    if (isErrorResult(result)) {
      // Store off the pending error - we use it to determine which loaders
      // to call and will commit it when we complete the navigation
      let boundaryMatch = skipLoaderErrorBubbling ? actionMatch : findNearestBoundary(matches, actionMatch.route.id);
      let context = await loadRouteData(loaderRequest, matches, requestContext, dataStrategy, skipLoaderErrorBubbling, null, [boundaryMatch.route.id, result]);
      // action status codes take precedence over loader status codes
      return _extends$2({}, context, {
        statusCode: isRouteErrorResponse(result.error) ? result.error.status : result.statusCode != null ? result.statusCode : 500,
        actionData: null,
        actionHeaders: _extends$2({}, result.headers ? {
          [actionMatch.route.id]: result.headers
        } : {})
      });
    }
    let context = await loadRouteData(loaderRequest, matches, requestContext, dataStrategy, skipLoaderErrorBubbling, null);
    return _extends$2({}, context, {
      actionData: {
        [actionMatch.route.id]: result.data
      }
    }, result.statusCode ? {
      statusCode: result.statusCode
    } : {}, {
      actionHeaders: result.headers ? {
        [actionMatch.route.id]: result.headers
      } : {}
    });
  }
  async function loadRouteData(request, matches, requestContext, dataStrategy, skipLoaderErrorBubbling, routeMatch, pendingActionResult) {
    let isRouteRequest = routeMatch != null;
    // Short circuit if we have no loaders to run (queryRoute())
    if (isRouteRequest && !(routeMatch != null && routeMatch.route.loader) && !(routeMatch != null && routeMatch.route.lazy)) {
      throw getInternalRouterError(400, {
        method: request.method,
        pathname: new URL(request.url).pathname,
        routeId: routeMatch == null ? void 0 : routeMatch.route.id
      });
    }
    let requestMatches = routeMatch ? [routeMatch] : pendingActionResult && isErrorResult(pendingActionResult[1]) ? getLoaderMatchesUntilBoundary(matches, pendingActionResult[0]) : matches;
    let matchesToLoad = requestMatches.filter(m => m.route.loader || m.route.lazy);
    // Short circuit if we have no loaders to run (query())
    if (matchesToLoad.length === 0) {
      return {
        matches,
        // Add a null for all matched routes for proper revalidation on the client
        loaderData: matches.reduce((acc, m) => Object.assign(acc, {
          [m.route.id]: null
        }), {}),
        errors: pendingActionResult && isErrorResult(pendingActionResult[1]) ? {
          [pendingActionResult[0]]: pendingActionResult[1].error
        } : null,
        statusCode: 200,
        loaderHeaders: {},
        activeDeferreds: null
      };
    }
    let results = await callDataStrategy("loader", request, matchesToLoad, matches, isRouteRequest, requestContext, dataStrategy);
    if (request.signal.aborted) {
      throwStaticHandlerAbortedError(request, isRouteRequest, future);
    }
    // Process and commit output from loaders
    let activeDeferreds = new Map();
    let context = processRouteLoaderData(matches, results, pendingActionResult, activeDeferreds, skipLoaderErrorBubbling);
    // Add a null for any non-loader matches for proper revalidation on the client
    let executedLoaders = new Set(matchesToLoad.map(match => match.route.id));
    matches.forEach(match => {
      if (!executedLoaders.has(match.route.id)) {
        context.loaderData[match.route.id] = null;
      }
    });
    return _extends$2({}, context, {
      matches,
      activeDeferreds: activeDeferreds.size > 0 ? Object.fromEntries(activeDeferreds.entries()) : null
    });
  }
  // Utility wrapper for calling dataStrategy server-side without having to
  // pass around the manifest, mapRouteProperties, etc.
  async function callDataStrategy(type, request, matchesToLoad, matches, isRouteRequest, requestContext, dataStrategy) {
    let results = await callDataStrategyImpl(dataStrategy || defaultDataStrategy, type, null, request, matchesToLoad, matches, null, manifest, mapRouteProperties, requestContext);
    let dataResults = {};
    await Promise.all(matches.map(async match => {
      if (!(match.route.id in results)) {
        return;
      }
      let result = results[match.route.id];
      if (isRedirectDataStrategyResultResult(result)) {
        let response = result.result;
        // Throw redirects and let the server handle them with an HTTP redirect
        throw normalizeRelativeRoutingRedirectResponse(response, request, match.route.id, matches, basename, future.v7_relativeSplatPath);
      }
      if (isResponse(result.result) && isRouteRequest) {
        // For SSR single-route requests, we want to hand Responses back
        // directly without unwrapping
        throw result;
      }
      dataResults[match.route.id] = await convertDataStrategyResultToDataResult(result);
    }));
    return dataResults;
  }
  return {
    dataRoutes,
    query,
    queryRoute
  };
}
//#endregion
////////////////////////////////////////////////////////////////////////////////
//#region Helpers
////////////////////////////////////////////////////////////////////////////////
/**
 * Given an existing StaticHandlerContext and an error thrown at render time,
 * provide an updated StaticHandlerContext suitable for a second SSR render
 */
function getStaticContextFromError(routes, context, error) {
  let newContext = _extends$2({}, context, {
    statusCode: isRouteErrorResponse(error) ? error.status : 500,
    errors: {
      [context._deepestRenderedBoundaryId || routes[0].id]: error
    }
  });
  return newContext;
}
function throwStaticHandlerAbortedError(request, isRouteRequest, future) {
  if (future.v7_throwAbortReason && request.signal.reason !== undefined) {
    throw request.signal.reason;
  }
  let method = isRouteRequest ? "queryRoute" : "query";
  throw new Error(method + "() call aborted: " + request.method + " " + request.url);
}
function isSubmissionNavigation(opts) {
  return opts != null && ("formData" in opts && opts.formData != null || "body" in opts && opts.body !== undefined);
}
function normalizeTo(location, matches, basename, prependBasename, to, v7_relativeSplatPath, fromRouteId, relative) {
  let contextualMatches;
  let activeRouteMatch;
  if (fromRouteId) {
    // Grab matches up to the calling route so our route-relative logic is
    // relative to the correct source route
    contextualMatches = [];
    for (let match of matches) {
      contextualMatches.push(match);
      if (match.route.id === fromRouteId) {
        activeRouteMatch = match;
        break;
      }
    }
  } else {
    contextualMatches = matches;
    activeRouteMatch = matches[matches.length - 1];
  }
  // Resolve the relative path
  let path = resolveTo(to ? to : ".", getResolveToMatches(contextualMatches, v7_relativeSplatPath), stripBasename(location.pathname, basename) || location.pathname, relative === "path");
  // When `to` is not specified we inherit search/hash from the current
  // location, unlike when to="." and we just inherit the path.
  // See https://github.com/remix-run/remix/issues/927
  if (to == null) {
    path.search = location.search;
    path.hash = location.hash;
  }
  // Account for `?index` params when routing to the current location
  if ((to == null || to === "" || to === ".") && activeRouteMatch) {
    let nakedIndex = hasNakedIndexQuery(path.search);
    if (activeRouteMatch.route.index && !nakedIndex) {
      // Add one when we're targeting an index route
      path.search = path.search ? path.search.replace(/^\?/, "?index&") : "?index";
    } else if (!activeRouteMatch.route.index && nakedIndex) {
      // Remove existing ones when we're not
      let params = new URLSearchParams(path.search);
      let indexValues = params.getAll("index");
      params.delete("index");
      indexValues.filter(v => v).forEach(v => params.append("index", v));
      let qs = params.toString();
      path.search = qs ? "?" + qs : "";
    }
  }
  // If we're operating within a basename, prepend it to the pathname.  If
  // this is a root navigation, then just use the raw basename which allows
  // the basename to have full control over the presence of a trailing slash
  // on root actions
  if (prependBasename && basename !== "/") {
    path.pathname = path.pathname === "/" ? basename : joinPaths([basename, path.pathname]);
  }
  return createPath(path);
}
// Normalize navigation options by converting formMethod=GET formData objects to
// URLSearchParams so they behave identically to links with query params
function normalizeNavigateOptions(normalizeFormMethod, isFetcher, path, opts) {
  // Return location verbatim on non-submission navigations
  if (!opts || !isSubmissionNavigation(opts)) {
    return {
      path
    };
  }
  if (opts.formMethod && !isValidMethod(opts.formMethod)) {
    return {
      path,
      error: getInternalRouterError(405, {
        method: opts.formMethod
      })
    };
  }
  let getInvalidBodyError = () => ({
    path,
    error: getInternalRouterError(400, {
      type: "invalid-body"
    })
  });
  // Create a Submission on non-GET navigations
  let rawFormMethod = opts.formMethod || "get";
  let formMethod = normalizeFormMethod ? rawFormMethod.toUpperCase() : rawFormMethod.toLowerCase();
  let formAction = stripHashFromPath(path);
  if (opts.body !== undefined) {
    if (opts.formEncType === "text/plain") {
      // text only support POST/PUT/PATCH/DELETE submissions
      if (!isMutationMethod(formMethod)) {
        return getInvalidBodyError();
      }
      let text = typeof opts.body === "string" ? opts.body : opts.body instanceof FormData || opts.body instanceof URLSearchParams ?
      // https://html.spec.whatwg.org/multipage/form-control-infrastructure.html#plain-text-form-data
      Array.from(opts.body.entries()).reduce((acc, _ref3) => {
        let [name, value] = _ref3;
        return "" + acc + name + "=" + value + "\n";
      }, "") : String(opts.body);
      return {
        path,
        submission: {
          formMethod,
          formAction,
          formEncType: opts.formEncType,
          formData: undefined,
          json: undefined,
          text
        }
      };
    } else if (opts.formEncType === "application/json") {
      // json only supports POST/PUT/PATCH/DELETE submissions
      if (!isMutationMethod(formMethod)) {
        return getInvalidBodyError();
      }
      try {
        let json = typeof opts.body === "string" ? JSON.parse(opts.body) : opts.body;
        return {
          path,
          submission: {
            formMethod,
            formAction,
            formEncType: opts.formEncType,
            formData: undefined,
            json,
            text: undefined
          }
        };
      } catch (e) {
        return getInvalidBodyError();
      }
    }
  }
  invariant(typeof FormData === "function", "FormData is not available in this environment");
  let searchParams;
  let formData;
  if (opts.formData) {
    searchParams = convertFormDataToSearchParams(opts.formData);
    formData = opts.formData;
  } else if (opts.body instanceof FormData) {
    searchParams = convertFormDataToSearchParams(opts.body);
    formData = opts.body;
  } else if (opts.body instanceof URLSearchParams) {
    searchParams = opts.body;
    formData = convertSearchParamsToFormData(searchParams);
  } else if (opts.body == null) {
    searchParams = new URLSearchParams();
    formData = new FormData();
  } else {
    try {
      searchParams = new URLSearchParams(opts.body);
      formData = convertSearchParamsToFormData(searchParams);
    } catch (e) {
      return getInvalidBodyError();
    }
  }
  let submission = {
    formMethod,
    formAction,
    formEncType: opts && opts.formEncType || "application/x-www-form-urlencoded",
    formData,
    json: undefined,
    text: undefined
  };
  if (isMutationMethod(submission.formMethod)) {
    return {
      path,
      submission
    };
  }
  // Flatten submission onto URLSearchParams for GET submissions
  let parsedPath = parsePath(path);
  // On GET navigation submissions we can drop the ?index param from the
  // resulting location since all loaders will run.  But fetcher GET submissions
  // only run a single loader so we need to preserve any incoming ?index params
  if (isFetcher && parsedPath.search && hasNakedIndexQuery(parsedPath.search)) {
    searchParams.append("index", "");
  }
  parsedPath.search = "?" + searchParams;
  return {
    path: createPath(parsedPath),
    submission
  };
}
// Filter out all routes at/below any caught error as they aren't going to
// render so we don't need to load them
function getLoaderMatchesUntilBoundary(matches, boundaryId, includeBoundary) {
  if (includeBoundary === void 0) {
    includeBoundary = false;
  }
  let index = matches.findIndex(m => m.route.id === boundaryId);
  if (index >= 0) {
    return matches.slice(0, includeBoundary ? index + 1 : index);
  }
  return matches;
}
function getMatchesToLoad(history, state, matches, submission, location, initialHydration, skipActionErrorRevalidation, isRevalidationRequired, cancelledDeferredRoutes, cancelledFetcherLoads, deletedFetchers, fetchLoadMatches, fetchRedirectIds, routesToUse, basename, pendingActionResult) {
  let actionResult = pendingActionResult ? isErrorResult(pendingActionResult[1]) ? pendingActionResult[1].error : pendingActionResult[1].data : undefined;
  let currentUrl = history.createURL(state.location);
  let nextUrl = history.createURL(location);
  // Pick navigation matches that are net-new or qualify for revalidation
  let boundaryMatches = matches;
  if (initialHydration && state.errors) {
    // On initial hydration, only consider matches up to _and including_ the boundary.
    // This is inclusive to handle cases where a server loader ran successfully,
    // a child server loader bubbled up to this route, but this route has
    // `clientLoader.hydrate` so we want to still run the `clientLoader` so that
    // we have a complete version of `loaderData`
    boundaryMatches = getLoaderMatchesUntilBoundary(matches, Object.keys(state.errors)[0], true);
  } else if (pendingActionResult && isErrorResult(pendingActionResult[1])) {
    // If an action threw an error, we call loaders up to, but not including the
    // boundary
    boundaryMatches = getLoaderMatchesUntilBoundary(matches, pendingActionResult[0]);
  }
  // Don't revalidate loaders by default after action 4xx/5xx responses
  // when the flag is enabled.  They can still opt-into revalidation via
  // `shouldRevalidate` via `actionResult`
  let actionStatus = pendingActionResult ? pendingActionResult[1].statusCode : undefined;
  let shouldSkipRevalidation = skipActionErrorRevalidation && actionStatus && actionStatus >= 400;
  let navigationMatches = boundaryMatches.filter((match, index) => {
    let {
      route
    } = match;
    if (route.lazy) {
      // We haven't loaded this route yet so we don't know if it's got a loader!
      return true;
    }
    if (route.loader == null) {
      return false;
    }
    if (initialHydration) {
      return shouldLoadRouteOnHydration(route, state.loaderData, state.errors);
    }
    // Always call the loader on new route instances and pending defer cancellations
    if (isNewLoader(state.loaderData, state.matches[index], match) || cancelledDeferredRoutes.some(id => id === match.route.id)) {
      return true;
    }
    // This is the default implementation for when we revalidate.  If the route
    // provides it's own implementation, then we give them full control but
    // provide this value so they can leverage it if needed after they check
    // their own specific use cases
    let currentRouteMatch = state.matches[index];
    let nextRouteMatch = match;
    return shouldRevalidateLoader(match, _extends$2({
      currentUrl,
      currentParams: currentRouteMatch.params,
      nextUrl,
      nextParams: nextRouteMatch.params
    }, submission, {
      actionResult,
      actionStatus,
      defaultShouldRevalidate: shouldSkipRevalidation ? false :
      // Forced revalidation due to submission, useRevalidator, or X-Remix-Revalidate
      isRevalidationRequired || currentUrl.pathname + currentUrl.search === nextUrl.pathname + nextUrl.search ||
      // Search params affect all loaders
      currentUrl.search !== nextUrl.search || isNewRouteInstance(currentRouteMatch, nextRouteMatch)
    }));
  });
  // Pick fetcher.loads that need to be revalidated
  let revalidatingFetchers = [];
  fetchLoadMatches.forEach((f, key) => {
    // Don't revalidate:
    //  - on initial hydration (shouldn't be any fetchers then anyway)
    //  - if fetcher won't be present in the subsequent render
    //    - no longer matches the URL (v7_fetcherPersist=false)
    //    - was unmounted but persisted due to v7_fetcherPersist=true
    if (initialHydration || !matches.some(m => m.route.id === f.routeId) || deletedFetchers.has(key)) {
      return;
    }
    let fetcherMatches = matchRoutes(routesToUse, f.path, basename);
    // If the fetcher path no longer matches, push it in with null matches so
    // we can trigger a 404 in callLoadersAndMaybeResolveData.  Note this is
    // currently only a use-case for Remix HMR where the route tree can change
    // at runtime and remove a route previously loaded via a fetcher
    if (!fetcherMatches) {
      revalidatingFetchers.push({
        key,
        routeId: f.routeId,
        path: f.path,
        matches: null,
        match: null,
        controller: null
      });
      return;
    }
    // Revalidating fetchers are decoupled from the route matches since they
    // load from a static href.  They revalidate based on explicit revalidation
    // (submission, useRevalidator, or X-Remix-Revalidate)
    let fetcher = state.fetchers.get(key);
    let fetcherMatch = getTargetMatch(fetcherMatches, f.path);
    let shouldRevalidate = false;
    if (fetchRedirectIds.has(key)) {
      // Never trigger a revalidation of an actively redirecting fetcher
      shouldRevalidate = false;
    } else if (cancelledFetcherLoads.has(key)) {
      // Always mark for revalidation if the fetcher was cancelled
      cancelledFetcherLoads.delete(key);
      shouldRevalidate = true;
    } else if (fetcher && fetcher.state !== "idle" && fetcher.data === undefined) {
      // If the fetcher hasn't ever completed loading yet, then this isn't a
      // revalidation, it would just be a brand new load if an explicit
      // revalidation is required
      shouldRevalidate = isRevalidationRequired;
    } else {
      // Otherwise fall back on any user-defined shouldRevalidate, defaulting
      // to explicit revalidations only
      shouldRevalidate = shouldRevalidateLoader(fetcherMatch, _extends$2({
        currentUrl,
        currentParams: state.matches[state.matches.length - 1].params,
        nextUrl,
        nextParams: matches[matches.length - 1].params
      }, submission, {
        actionResult,
        actionStatus,
        defaultShouldRevalidate: shouldSkipRevalidation ? false : isRevalidationRequired
      }));
    }
    if (shouldRevalidate) {
      revalidatingFetchers.push({
        key,
        routeId: f.routeId,
        path: f.path,
        matches: fetcherMatches,
        match: fetcherMatch,
        controller: new AbortController()
      });
    }
  });
  return [navigationMatches, revalidatingFetchers];
}
function shouldLoadRouteOnHydration(route, loaderData, errors) {
  // We dunno if we have a loader - gotta find out!
  if (route.lazy) {
    return true;
  }
  // No loader, nothing to initialize
  if (!route.loader) {
    return false;
  }
  let hasData = loaderData != null && loaderData[route.id] !== undefined;
  let hasError = errors != null && errors[route.id] !== undefined;
  // Don't run if we error'd during SSR
  if (!hasData && hasError) {
    return false;
  }
  // Explicitly opting-in to running on hydration
  if (typeof route.loader === "function" && route.loader.hydrate === true) {
    return true;
  }
  // Otherwise, run if we're not yet initialized with anything
  return !hasData && !hasError;
}
function isNewLoader(currentLoaderData, currentMatch, match) {
  let isNew =
  // [a] -> [a, b]
  !currentMatch ||
  // [a, b] -> [a, c]
  match.route.id !== currentMatch.route.id;
  // Handle the case that we don't have data for a re-used route, potentially
  // from a prior error or from a cancelled pending deferred
  let isMissingData = currentLoaderData[match.route.id] === undefined;
  // Always load if this is a net-new route or we don't yet have data
  return isNew || isMissingData;
}
function isNewRouteInstance(currentMatch, match) {
  let currentPath = currentMatch.route.path;
  return (
    // param change for this match, /users/123 -> /users/456
    currentMatch.pathname !== match.pathname ||
    // splat param changed, which is not present in match.path
    // e.g. /files/images/avatar.jpg -> files/finances.xls
    currentPath != null && currentPath.endsWith("*") && currentMatch.params["*"] !== match.params["*"]
  );
}
function shouldRevalidateLoader(loaderMatch, arg) {
  if (loaderMatch.route.shouldRevalidate) {
    let routeChoice = loaderMatch.route.shouldRevalidate(arg);
    if (typeof routeChoice === "boolean") {
      return routeChoice;
    }
  }
  return arg.defaultShouldRevalidate;
}
function patchRoutesImpl(routeId, children, routesToUse, manifest, mapRouteProperties) {
  var _childrenToPatch;
  let childrenToPatch;
  if (routeId) {
    let route = manifest[routeId];
    invariant(route, "No route found to patch children into: routeId = " + routeId);
    if (!route.children) {
      route.children = [];
    }
    childrenToPatch = route.children;
  } else {
    childrenToPatch = routesToUse;
  }
  // Don't patch in routes we already know about so that `patch` is idempotent
  // to simplify user-land code. This is useful because we re-call the
  // `patchRoutesOnNavigation` function for matched routes with params.
  let uniqueChildren = children.filter(newRoute => !childrenToPatch.some(existingRoute => isSameRoute(newRoute, existingRoute)));
  let newRoutes = convertRoutesToDataRoutes(uniqueChildren, mapRouteProperties, [routeId || "_", "patch", String(((_childrenToPatch = childrenToPatch) == null ? void 0 : _childrenToPatch.length) || "0")], manifest);
  childrenToPatch.push(...newRoutes);
}
function isSameRoute(newRoute, existingRoute) {
  // Most optimal check is by id
  if ("id" in newRoute && "id" in existingRoute && newRoute.id === existingRoute.id) {
    return true;
  }
  // Second is by pathing differences
  if (!(newRoute.index === existingRoute.index && newRoute.path === existingRoute.path && newRoute.caseSensitive === existingRoute.caseSensitive)) {
    return false;
  }
  // Pathless layout routes are trickier since we need to check children.
  // If they have no children then they're the same as far as we can tell
  if ((!newRoute.children || newRoute.children.length === 0) && (!existingRoute.children || existingRoute.children.length === 0)) {
    return true;
  }
  // Otherwise, we look to see if every child in the new route is already
  // represented in the existing route's children
  return newRoute.children.every((aChild, i) => {
    var _existingRoute$childr;
    return (_existingRoute$childr = existingRoute.children) == null ? void 0 : _existingRoute$childr.some(bChild => isSameRoute(aChild, bChild));
  });
}
/**
 * Execute route.lazy() methods to lazily load route modules (loader, action,
 * shouldRevalidate) and update the routeManifest in place which shares objects
 * with dataRoutes so those get updated as well.
 */
async function loadLazyRouteModule(route, mapRouteProperties, manifest) {
  if (!route.lazy) {
    return;
  }
  let lazyRoute = await route.lazy();
  // If the lazy route function was executed and removed by another parallel
  // call then we can return - first lazy() to finish wins because the return
  // value of lazy is expected to be static
  if (!route.lazy) {
    return;
  }
  let routeToUpdate = manifest[route.id];
  invariant(routeToUpdate, "No route found in manifest");
  // Update the route in place.  This should be safe because there's no way
  // we could yet be sitting on this route as we can't get there without
  // resolving lazy() first.
  //
  // This is different than the HMR "update" use-case where we may actively be
  // on the route being updated.  The main concern boils down to "does this
  // mutation affect any ongoing navigations or any current state.matches
  // values?".  If not, it should be safe to update in place.
  let routeUpdates = {};
  for (let lazyRouteProperty in lazyRoute) {
    let staticRouteValue = routeToUpdate[lazyRouteProperty];
    let isPropertyStaticallyDefined = staticRouteValue !== undefined &&
    // This property isn't static since it should always be updated based
    // on the route updates
    lazyRouteProperty !== "hasErrorBoundary";
    warning(!isPropertyStaticallyDefined, "Route \"" + routeToUpdate.id + "\" has a static property \"" + lazyRouteProperty + "\" " + "defined but its lazy function is also returning a value for this property. " + ("The lazy route property \"" + lazyRouteProperty + "\" will be ignored."));
    if (!isPropertyStaticallyDefined && !immutableRouteKeys.has(lazyRouteProperty)) {
      routeUpdates[lazyRouteProperty] = lazyRoute[lazyRouteProperty];
    }
  }
  // Mutate the route with the provided updates.  Do this first so we pass
  // the updated version to mapRouteProperties
  Object.assign(routeToUpdate, routeUpdates);
  // Mutate the `hasErrorBoundary` property on the route based on the route
  // updates and remove the `lazy` function so we don't resolve the lazy
  // route again.
  Object.assign(routeToUpdate, _extends$2({}, mapRouteProperties(routeToUpdate), {
    lazy: undefined
  }));
}
// Default implementation of `dataStrategy` which fetches all loaders in parallel
async function defaultDataStrategy(_ref4) {
  let {
    matches
  } = _ref4;
  let matchesToLoad = matches.filter(m => m.shouldLoad);
  let results = await Promise.all(matchesToLoad.map(m => m.resolve()));
  return results.reduce((acc, result, i) => Object.assign(acc, {
    [matchesToLoad[i].route.id]: result
  }), {});
}
async function callDataStrategyImpl(dataStrategyImpl, type, state, request, matchesToLoad, matches, fetcherKey, manifest, mapRouteProperties, requestContext) {
  let loadRouteDefinitionsPromises = matches.map(m => m.route.lazy ? loadLazyRouteModule(m.route, mapRouteProperties, manifest) : undefined);
  let dsMatches = matches.map((match, i) => {
    let loadRoutePromise = loadRouteDefinitionsPromises[i];
    let shouldLoad = matchesToLoad.some(m => m.route.id === match.route.id);
    // `resolve` encapsulates route.lazy(), executing the loader/action,
    // and mapping return values/thrown errors to a `DataStrategyResult`.  Users
    // can pass a callback to take fine-grained control over the execution
    // of the loader/action
    let resolve = async handlerOverride => {
      if (handlerOverride && request.method === "GET" && (match.route.lazy || match.route.loader)) {
        shouldLoad = true;
      }
      return shouldLoad ? callLoaderOrAction(type, request, match, loadRoutePromise, handlerOverride, requestContext) : Promise.resolve({
        type: ResultType.data,
        result: undefined
      });
    };
    return _extends$2({}, match, {
      shouldLoad,
      resolve
    });
  });
  // Send all matches here to allow for a middleware-type implementation.
  // handler will be a no-op for unneeded routes and we filter those results
  // back out below.
  let results = await dataStrategyImpl({
    matches: dsMatches,
    request,
    params: matches[0].params,
    fetcherKey,
    context: requestContext
  });
  // Wait for all routes to load here but 'swallow the error since we want
  // it to bubble up from the `await loadRoutePromise` in `callLoaderOrAction` -
  // called from `match.resolve()`
  try {
    await Promise.all(loadRouteDefinitionsPromises);
  } catch (e) {
    // No-op
  }
  return results;
}
// Default logic for calling a loader/action is the user has no specified a dataStrategy
async function callLoaderOrAction(type, request, match, loadRoutePromise, handlerOverride, staticContext) {
  let result;
  let onReject;
  let runHandler = handler => {
    // Setup a promise we can race against so that abort signals short circuit
    let reject;
    // This will never resolve so safe to type it as Promise<DataStrategyResult> to
    // satisfy the function return value
    let abortPromise = new Promise((_, r) => reject = r);
    onReject = () => reject();
    request.signal.addEventListener("abort", onReject);
    let actualHandler = ctx => {
      if (typeof handler !== "function") {
        return Promise.reject(new Error("You cannot call the handler for a route which defines a boolean " + ("\"" + type + "\" [routeId: " + match.route.id + "]")));
      }
      return handler({
        request,
        params: match.params,
        context: staticContext
      }, ...(ctx !== undefined ? [ctx] : []));
    };
    let handlerPromise = (async () => {
      try {
        let val = await (handlerOverride ? handlerOverride(ctx => actualHandler(ctx)) : actualHandler());
        return {
          type: "data",
          result: val
        };
      } catch (e) {
        return {
          type: "error",
          result: e
        };
      }
    })();
    return Promise.race([handlerPromise, abortPromise]);
  };
  try {
    let handler = match.route[type];
    // If we have a route.lazy promise, await that first
    if (loadRoutePromise) {
      if (handler) {
        // Run statically defined handler in parallel with lazy()
        let handlerError;
        let [value] = await Promise.all([
        // If the handler throws, don't let it immediately bubble out,
        // since we need to let the lazy() execution finish so we know if this
        // route has a boundary that can handle the error
        runHandler(handler).catch(e => {
          handlerError = e;
        }), loadRoutePromise]);
        if (handlerError !== undefined) {
          throw handlerError;
        }
        result = value;
      } else {
        // Load lazy route module, then run any returned handler
        await loadRoutePromise;
        handler = match.route[type];
        if (handler) {
          // Handler still runs even if we got interrupted to maintain consistency
          // with un-abortable behavior of handler execution on non-lazy or
          // previously-lazy-loaded routes
          result = await runHandler(handler);
        } else if (type === "action") {
          let url = new URL(request.url);
          let pathname = url.pathname + url.search;
          throw getInternalRouterError(405, {
            method: request.method,
            pathname,
            routeId: match.route.id
          });
        } else {
          // lazy() route has no loader to run.  Short circuit here so we don't
          // hit the invariant below that errors on returning undefined.
          return {
            type: ResultType.data,
            result: undefined
          };
        }
      }
    } else if (!handler) {
      let url = new URL(request.url);
      let pathname = url.pathname + url.search;
      throw getInternalRouterError(404, {
        pathname
      });
    } else {
      result = await runHandler(handler);
    }
    invariant(result.result !== undefined, "You defined " + (type === "action" ? "an action" : "a loader") + " for route " + ("\"" + match.route.id + "\" but didn't return anything from your `" + type + "` ") + "function. Please return a value or `null`.");
  } catch (e) {
    // We should already be catching and converting normal handler executions to
    // DataStrategyResults and returning them, so anything that throws here is an
    // unexpected error we still need to wrap
    return {
      type: ResultType.error,
      result: e
    };
  } finally {
    if (onReject) {
      request.signal.removeEventListener("abort", onReject);
    }
  }
  return result;
}
async function convertDataStrategyResultToDataResult(dataStrategyResult) {
  let {
    result,
    type
  } = dataStrategyResult;
  if (isResponse(result)) {
    let data;
    try {
      let contentType = result.headers.get("Content-Type");
      // Check between word boundaries instead of startsWith() due to the last
      // paragraph of https://httpwg.org/specs/rfc9110.html#field.content-type
      if (contentType && /\bapplication\/json\b/.test(contentType)) {
        if (result.body == null) {
          data = null;
        } else {
          data = await result.json();
        }
      } else {
        data = await result.text();
      }
    } catch (e) {
      return {
        type: ResultType.error,
        error: e
      };
    }
    if (type === ResultType.error) {
      return {
        type: ResultType.error,
        error: new ErrorResponseImpl(result.status, result.statusText, data),
        statusCode: result.status,
        headers: result.headers
      };
    }
    return {
      type: ResultType.data,
      data,
      statusCode: result.status,
      headers: result.headers
    };
  }
  if (type === ResultType.error) {
    if (isDataWithResponseInit(result)) {
      var _result$init3, _result$init4;
      if (result.data instanceof Error) {
        var _result$init, _result$init2;
        return {
          type: ResultType.error,
          error: result.data,
          statusCode: (_result$init = result.init) == null ? void 0 : _result$init.status,
          headers: (_result$init2 = result.init) != null && _result$init2.headers ? new Headers(result.init.headers) : undefined
        };
      }
      // Convert thrown data() to ErrorResponse instances
      return {
        type: ResultType.error,
        error: new ErrorResponseImpl(((_result$init3 = result.init) == null ? void 0 : _result$init3.status) || 500, undefined, result.data),
        statusCode: isRouteErrorResponse(result) ? result.status : undefined,
        headers: (_result$init4 = result.init) != null && _result$init4.headers ? new Headers(result.init.headers) : undefined
      };
    }
    return {
      type: ResultType.error,
      error: result,
      statusCode: isRouteErrorResponse(result) ? result.status : undefined
    };
  }
  if (isDeferredData(result)) {
    var _result$init5, _result$init6;
    return {
      type: ResultType.deferred,
      deferredData: result,
      statusCode: (_result$init5 = result.init) == null ? void 0 : _result$init5.status,
      headers: ((_result$init6 = result.init) == null ? void 0 : _result$init6.headers) && new Headers(result.init.headers)
    };
  }
  if (isDataWithResponseInit(result)) {
    var _result$init7, _result$init8;
    return {
      type: ResultType.data,
      data: result.data,
      statusCode: (_result$init7 = result.init) == null ? void 0 : _result$init7.status,
      headers: (_result$init8 = result.init) != null && _result$init8.headers ? new Headers(result.init.headers) : undefined
    };
  }
  return {
    type: ResultType.data,
    data: result
  };
}
// Support relative routing in internal redirects
function normalizeRelativeRoutingRedirectResponse(response, request, routeId, matches, basename, v7_relativeSplatPath) {
  let location = response.headers.get("Location");
  invariant(location, "Redirects returned/thrown from loaders/actions must have a Location header");
  if (!ABSOLUTE_URL_REGEX$2.test(location)) {
    let trimmedMatches = matches.slice(0, matches.findIndex(m => m.route.id === routeId) + 1);
    location = normalizeTo(new URL(request.url), trimmedMatches, basename, true, location, v7_relativeSplatPath);
    response.headers.set("Location", location);
  }
  return response;
}
function normalizeRedirectLocation(location, currentUrl, basename) {
  if (ABSOLUTE_URL_REGEX$2.test(location)) {
    // Strip off the protocol+origin for same-origin + same-basename absolute redirects
    let normalizedLocation = location;
    let url = normalizedLocation.startsWith("//") ? new URL(currentUrl.protocol + normalizedLocation) : new URL(normalizedLocation);
    let isSameBasename = stripBasename(url.pathname, basename) != null;
    if (url.origin === currentUrl.origin && isSameBasename) {
      return url.pathname + url.search + url.hash;
    }
  }
  return location;
}
// Utility method for creating the Request instances for loaders/actions during
// client-side navigations and fetches.  During SSR we will always have a
// Request instance from the static handler (query/queryRoute)
function createClientSideRequest(history, location, signal, submission) {
  let url = history.createURL(stripHashFromPath(location)).toString();
  let init = {
    signal
  };
  if (submission && isMutationMethod(submission.formMethod)) {
    let {
      formMethod,
      formEncType
    } = submission;
    // Didn't think we needed this but it turns out unlike other methods, patch
    // won't be properly normalized to uppercase and results in a 405 error.
    // See: https://fetch.spec.whatwg.org/#concept-method
    init.method = formMethod.toUpperCase();
    if (formEncType === "application/json") {
      init.headers = new Headers({
        "Content-Type": formEncType
      });
      init.body = JSON.stringify(submission.json);
    } else if (formEncType === "text/plain") {
      // Content-Type is inferred (https://fetch.spec.whatwg.org/#dom-request)
      init.body = submission.text;
    } else if (formEncType === "application/x-www-form-urlencoded" && submission.formData) {
      // Content-Type is inferred (https://fetch.spec.whatwg.org/#dom-request)
      init.body = convertFormDataToSearchParams(submission.formData);
    } else {
      // Content-Type is inferred (https://fetch.spec.whatwg.org/#dom-request)
      init.body = submission.formData;
    }
  }
  return new Request(url, init);
}
function convertFormDataToSearchParams(formData) {
  let searchParams = new URLSearchParams();
  for (let [key, value] of formData.entries()) {
    // https://html.spec.whatwg.org/multipage/form-control-infrastructure.html#converting-an-entry-list-to-a-list-of-name-value-pairs
    searchParams.append(key, typeof value === "string" ? value : value.name);
  }
  return searchParams;
}
function convertSearchParamsToFormData(searchParams) {
  let formData = new FormData();
  for (let [key, value] of searchParams.entries()) {
    formData.append(key, value);
  }
  return formData;
}
function processRouteLoaderData(matches, results, pendingActionResult, activeDeferreds, skipLoaderErrorBubbling) {
  // Fill in loaderData/errors from our loaders
  let loaderData = {};
  let errors = null;
  let statusCode;
  let foundError = false;
  let loaderHeaders = {};
  let pendingError = pendingActionResult && isErrorResult(pendingActionResult[1]) ? pendingActionResult[1].error : undefined;
  // Process loader results into state.loaderData/state.errors
  matches.forEach(match => {
    if (!(match.route.id in results)) {
      return;
    }
    let id = match.route.id;
    let result = results[id];
    invariant(!isRedirectResult(result), "Cannot handle redirect results in processLoaderData");
    if (isErrorResult(result)) {
      let error = result.error;
      // If we have a pending action error, we report it at the highest-route
      // that throws a loader error, and then clear it out to indicate that
      // it was consumed
      if (pendingError !== undefined) {
        error = pendingError;
        pendingError = undefined;
      }
      errors = errors || {};
      if (skipLoaderErrorBubbling) {
        errors[id] = error;
      } else {
        // Look upwards from the matched route for the closest ancestor error
        // boundary, defaulting to the root match.  Prefer higher error values
        // if lower errors bubble to the same boundary
        let boundaryMatch = findNearestBoundary(matches, id);
        if (errors[boundaryMatch.route.id] == null) {
          errors[boundaryMatch.route.id] = error;
        }
      }
      // Clear our any prior loaderData for the throwing route
      loaderData[id] = undefined;
      // Once we find our first (highest) error, we set the status code and
      // prevent deeper status codes from overriding
      if (!foundError) {
        foundError = true;
        statusCode = isRouteErrorResponse(result.error) ? result.error.status : 500;
      }
      if (result.headers) {
        loaderHeaders[id] = result.headers;
      }
    } else {
      if (isDeferredResult(result)) {
        activeDeferreds.set(id, result.deferredData);
        loaderData[id] = result.deferredData.data;
        // Error status codes always override success status codes, but if all
        // loaders are successful we take the deepest status code.
        if (result.statusCode != null && result.statusCode !== 200 && !foundError) {
          statusCode = result.statusCode;
        }
        if (result.headers) {
          loaderHeaders[id] = result.headers;
        }
      } else {
        loaderData[id] = result.data;
        // Error status codes always override success status codes, but if all
        // loaders are successful we take the deepest status code.
        if (result.statusCode && result.statusCode !== 200 && !foundError) {
          statusCode = result.statusCode;
        }
        if (result.headers) {
          loaderHeaders[id] = result.headers;
        }
      }
    }
  });
  // If we didn't consume the pending action error (i.e., all loaders
  // resolved), then consume it here.  Also clear out any loaderData for the
  // throwing route
  if (pendingError !== undefined && pendingActionResult) {
    errors = {
      [pendingActionResult[0]]: pendingError
    };
    loaderData[pendingActionResult[0]] = undefined;
  }
  return {
    loaderData,
    errors,
    statusCode: statusCode || 200,
    loaderHeaders
  };
}
function processLoaderData(state, matches, results, pendingActionResult, revalidatingFetchers, fetcherResults, activeDeferreds) {
  let {
    loaderData,
    errors
  } = processRouteLoaderData(matches, results, pendingActionResult, activeDeferreds, false // This method is only called client side so we always want to bubble
  );
  // Process results from our revalidating fetchers
  revalidatingFetchers.forEach(rf => {
    let {
      key,
      match,
      controller
    } = rf;
    let result = fetcherResults[key];
    invariant(result, "Did not find corresponding fetcher result");
    // Process fetcher non-redirect errors
    if (controller && controller.signal.aborted) {
      // Nothing to do for aborted fetchers
      return;
    } else if (isErrorResult(result)) {
      let boundaryMatch = findNearestBoundary(state.matches, match == null ? void 0 : match.route.id);
      if (!(errors && errors[boundaryMatch.route.id])) {
        errors = _extends$2({}, errors, {
          [boundaryMatch.route.id]: result.error
        });
      }
      state.fetchers.delete(key);
    } else if (isRedirectResult(result)) {
      // Should never get here, redirects should get processed above, but we
      // keep this to type narrow to a success result in the else
      invariant(false, "Unhandled fetcher revalidation redirect");
    } else if (isDeferredResult(result)) {
      // Should never get here, deferred data should be awaited for fetchers
      // in resolveDeferredResults
      invariant(false, "Unhandled fetcher deferred data");
    } else {
      let doneFetcher = getDoneFetcher(result.data);
      state.fetchers.set(key, doneFetcher);
    }
  });
  return {
    loaderData,
    errors
  };
}
function mergeLoaderData(loaderData, newLoaderData, matches, errors) {
  let mergedLoaderData = _extends$2({}, newLoaderData);
  for (let match of matches) {
    let id = match.route.id;
    if (newLoaderData.hasOwnProperty(id)) {
      if (newLoaderData[id] !== undefined) {
        mergedLoaderData[id] = newLoaderData[id];
      }
    } else if (loaderData[id] !== undefined && match.route.loader) {
      // Preserve existing keys not included in newLoaderData and where a loader
      // wasn't removed by HMR
      mergedLoaderData[id] = loaderData[id];
    }
    if (errors && errors.hasOwnProperty(id)) {
      // Don't keep any loader data below the boundary
      break;
    }
  }
  return mergedLoaderData;
}
function getActionDataForCommit(pendingActionResult) {
  if (!pendingActionResult) {
    return {};
  }
  return isErrorResult(pendingActionResult[1]) ? {
    // Clear out prior actionData on errors
    actionData: {}
  } : {
    actionData: {
      [pendingActionResult[0]]: pendingActionResult[1].data
    }
  };
}
// Find the nearest error boundary, looking upwards from the leaf route (or the
// route specified by routeId) for the closest ancestor error boundary,
// defaulting to the root match
function findNearestBoundary(matches, routeId) {
  let eligibleMatches = routeId ? matches.slice(0, matches.findIndex(m => m.route.id === routeId) + 1) : [...matches];
  return eligibleMatches.reverse().find(m => m.route.hasErrorBoundary === true) || matches[0];
}
function getShortCircuitMatches(routes) {
  // Prefer a root layout route if present, otherwise shim in a route object
  let route = routes.length === 1 ? routes[0] : routes.find(r => r.index || !r.path || r.path === "/") || {
    id: "__shim-error-route__"
  };
  return {
    matches: [{
      params: {},
      pathname: "",
      pathnameBase: "",
      route
    }],
    route
  };
}
function getInternalRouterError(status, _temp5) {
  let {
    pathname,
    routeId,
    method,
    type,
    message
  } = _temp5 === void 0 ? {} : _temp5;
  let statusText = "Unknown Server Error";
  let errorMessage = "Unknown @remix-run/router error";
  if (status === 400) {
    statusText = "Bad Request";
    if (method && pathname && routeId) {
      errorMessage = "You made a " + method + " request to \"" + pathname + "\" but " + ("did not provide a `loader` for route \"" + routeId + "\", ") + "so there is no way to handle the request.";
    } else if (type === "defer-action") {
      errorMessage = "defer() is not supported in actions";
    } else if (type === "invalid-body") {
      errorMessage = "Unable to encode submission body";
    }
  } else if (status === 403) {
    statusText = "Forbidden";
    errorMessage = "Route \"" + routeId + "\" does not match URL \"" + pathname + "\"";
  } else if (status === 404) {
    statusText = "Not Found";
    errorMessage = "No route matches URL \"" + pathname + "\"";
  } else if (status === 405) {
    statusText = "Method Not Allowed";
    if (method && pathname && routeId) {
      errorMessage = "You made a " + method.toUpperCase() + " request to \"" + pathname + "\" but " + ("did not provide an `action` for route \"" + routeId + "\", ") + "so there is no way to handle the request.";
    } else if (method) {
      errorMessage = "Invalid request method \"" + method.toUpperCase() + "\"";
    }
  }
  return new ErrorResponseImpl(status || 500, statusText, new Error(errorMessage), true);
}
// Find any returned redirect errors, starting from the lowest match
function findRedirect(results) {
  let entries = Object.entries(results);
  for (let i = entries.length - 1; i >= 0; i--) {
    let [key, result] = entries[i];
    if (isRedirectResult(result)) {
      return {
        key,
        result
      };
    }
  }
}
function stripHashFromPath(path) {
  let parsedPath = typeof path === "string" ? parsePath(path) : path;
  return createPath(_extends$2({}, parsedPath, {
    hash: ""
  }));
}
function isHashChangeOnly(a, b) {
  if (a.pathname !== b.pathname || a.search !== b.search) {
    return false;
  }
  if (a.hash === "") {
    // /page -> /page#hash
    return b.hash !== "";
  } else if (a.hash === b.hash) {
    // /page#hash -> /page#hash
    return true;
  } else if (b.hash !== "") {
    // /page#hash -> /page#other
    return true;
  }
  // If the hash is removed the browser will re-perform a request to the server
  // /page#hash -> /page
  return false;
}
function isDataStrategyResult(result) {
  return result != null && typeof result === "object" && "type" in result && "result" in result && (result.type === ResultType.data || result.type === ResultType.error);
}
function isRedirectDataStrategyResultResult(result) {
  return isResponse(result.result) && redirectStatusCodes.has(result.result.status);
}
function isDeferredResult(result) {
  return result.type === ResultType.deferred;
}
function isErrorResult(result) {
  return result.type === ResultType.error;
}
function isRedirectResult(result) {
  return (result && result.type) === ResultType.redirect;
}
function isDataWithResponseInit(value) {
  return typeof value === "object" && value != null && "type" in value && "data" in value && "init" in value && value.type === "DataWithResponseInit";
}
function isDeferredData(value) {
  let deferred = value;
  return deferred && typeof deferred === "object" && typeof deferred.data === "object" && typeof deferred.subscribe === "function" && typeof deferred.cancel === "function" && typeof deferred.resolveData === "function";
}
function isResponse(value) {
  return value != null && typeof value.status === "number" && typeof value.statusText === "string" && typeof value.headers === "object" && typeof value.body !== "undefined";
}
function isRedirectResponse(result) {
  if (!isResponse(result)) {
    return false;
  }
  let status = result.status;
  let location = result.headers.get("Location");
  return status >= 300 && status <= 399 && location != null;
}
function isValidMethod(method) {
  return validRequestMethods.has(method.toLowerCase());
}
function isMutationMethod(method) {
  return validMutationMethods.has(method.toLowerCase());
}
async function resolveNavigationDeferredResults(matches, results, signal, currentMatches, currentLoaderData) {
  let entries = Object.entries(results);
  for (let index = 0; index < entries.length; index++) {
    let [routeId, result] = entries[index];
    let match = matches.find(m => (m == null ? void 0 : m.route.id) === routeId);
    // If we don't have a match, then we can have a deferred result to do
    // anything with.  This is for revalidating fetchers where the route was
    // removed during HMR
    if (!match) {
      continue;
    }
    let currentMatch = currentMatches.find(m => m.route.id === match.route.id);
    let isRevalidatingLoader = currentMatch != null && !isNewRouteInstance(currentMatch, match) && (currentLoaderData && currentLoaderData[match.route.id]) !== undefined;
    if (isDeferredResult(result) && isRevalidatingLoader) {
      // Note: we do not have to touch activeDeferreds here since we race them
      // against the signal in resolveDeferredData and they'll get aborted
      // there if needed
      await resolveDeferredData(result, signal, false).then(result => {
        if (result) {
          results[routeId] = result;
        }
      });
    }
  }
}
async function resolveFetcherDeferredResults(matches, results, revalidatingFetchers) {
  for (let index = 0; index < revalidatingFetchers.length; index++) {
    let {
      key,
      routeId,
      controller
    } = revalidatingFetchers[index];
    let result = results[key];
    let match = matches.find(m => (m == null ? void 0 : m.route.id) === routeId);
    // If we don't have a match, then we can have a deferred result to do
    // anything with.  This is for revalidating fetchers where the route was
    // removed during HMR
    if (!match) {
      continue;
    }
    if (isDeferredResult(result)) {
      // Note: we do not have to touch activeDeferreds here since we race them
      // against the signal in resolveDeferredData and they'll get aborted
      // there if needed
      invariant(controller, "Expected an AbortController for revalidating fetcher deferred result");
      await resolveDeferredData(result, controller.signal, true).then(result => {
        if (result) {
          results[key] = result;
        }
      });
    }
  }
}
async function resolveDeferredData(result, signal, unwrap) {
  if (unwrap === void 0) {
    unwrap = false;
  }
  let aborted = await result.deferredData.resolveData(signal);
  if (aborted) {
    return;
  }
  if (unwrap) {
    try {
      return {
        type: ResultType.data,
        data: result.deferredData.unwrappedData
      };
    } catch (e) {
      // Handle any TrackedPromise._error values encountered while unwrapping
      return {
        type: ResultType.error,
        error: e
      };
    }
  }
  return {
    type: ResultType.data,
    data: result.deferredData.data
  };
}
function hasNakedIndexQuery(search) {
  return new URLSearchParams(search).getAll("index").some(v => v === "");
}
function getTargetMatch(matches, location) {
  let search = typeof location === "string" ? parsePath(location).search : location.search;
  if (matches[matches.length - 1].route.index && hasNakedIndexQuery(search || "")) {
    // Return the leaf index route when index is present
    return matches[matches.length - 1];
  }
  // Otherwise grab the deepest "path contributing" match (ignoring index and
  // pathless layout routes)
  let pathMatches = getPathContributingMatches(matches);
  return pathMatches[pathMatches.length - 1];
}
function getSubmissionFromNavigation(navigation) {
  let {
    formMethod,
    formAction,
    formEncType,
    text,
    formData,
    json
  } = navigation;
  if (!formMethod || !formAction || !formEncType) {
    return;
  }
  if (text != null) {
    return {
      formMethod,
      formAction,
      formEncType,
      formData: undefined,
      json: undefined,
      text
    };
  } else if (formData != null) {
    return {
      formMethod,
      formAction,
      formEncType,
      formData,
      json: undefined,
      text: undefined
    };
  } else if (json !== undefined) {
    return {
      formMethod,
      formAction,
      formEncType,
      formData: undefined,
      json,
      text: undefined
    };
  }
}
function getLoadingNavigation(location, submission) {
  if (submission) {
    let navigation = {
      state: "loading",
      location,
      formMethod: submission.formMethod,
      formAction: submission.formAction,
      formEncType: submission.formEncType,
      formData: submission.formData,
      json: submission.json,
      text: submission.text
    };
    return navigation;
  } else {
    let navigation = {
      state: "loading",
      location,
      formMethod: undefined,
      formAction: undefined,
      formEncType: undefined,
      formData: undefined,
      json: undefined,
      text: undefined
    };
    return navigation;
  }
}
function getSubmittingNavigation(location, submission) {
  let navigation = {
    state: "submitting",
    location,
    formMethod: submission.formMethod,
    formAction: submission.formAction,
    formEncType: submission.formEncType,
    formData: submission.formData,
    json: submission.json,
    text: submission.text
  };
  return navigation;
}
function getLoadingFetcher(submission, data) {
  if (submission) {
    let fetcher = {
      state: "loading",
      formMethod: submission.formMethod,
      formAction: submission.formAction,
      formEncType: submission.formEncType,
      formData: submission.formData,
      json: submission.json,
      text: submission.text,
      data
    };
    return fetcher;
  } else {
    let fetcher = {
      state: "loading",
      formMethod: undefined,
      formAction: undefined,
      formEncType: undefined,
      formData: undefined,
      json: undefined,
      text: undefined,
      data
    };
    return fetcher;
  }
}
function getSubmittingFetcher(submission, existingFetcher) {
  let fetcher = {
    state: "submitting",
    formMethod: submission.formMethod,
    formAction: submission.formAction,
    formEncType: submission.formEncType,
    formData: submission.formData,
    json: submission.json,
    text: submission.text,
    data: existingFetcher ? existingFetcher.data : undefined
  };
  return fetcher;
}
function getDoneFetcher(data) {
  let fetcher = {
    state: "idle",
    formMethod: undefined,
    formAction: undefined,
    formEncType: undefined,
    formData: undefined,
    json: undefined,
    text: undefined,
    data
  };
  return fetcher;
}
function restoreAppliedTransitions(_window, transitions) {
  try {
    let sessionPositions = _window.sessionStorage.getItem(TRANSITIONS_STORAGE_KEY);
    if (sessionPositions) {
      let json = JSON.parse(sessionPositions);
      for (let [k, v] of Object.entries(json || {})) {
        if (v && Array.isArray(v)) {
          transitions.set(k, new Set(v || []));
        }
      }
    }
  } catch (e) {
    // no-op, use default empty object
  }
}
function persistAppliedTransitions(_window, transitions) {
  if (transitions.size > 0) {
    let json = {};
    for (let [k, v] of transitions) {
      json[k] = [...v];
    }
    try {
      _window.sessionStorage.setItem(TRANSITIONS_STORAGE_KEY, JSON.stringify(json));
    } catch (error) {
      warning(false, "Failed to save applied view transitions in sessionStorage (" + error + ").");
    }
  }
}

/**
 * React Router v6.30.2
 *
 * Copyright (c) Remix Software Inc.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE.md file in the root directory of this source tree.
 *
 * @license MIT
 */
function _extends$1() {
  _extends$1 = Object.assign ? Object.assign.bind() : function(target) {
    for (var i = 1; i < arguments.length; i++) {
      var source = arguments[i];
      for (var key in source) {
        if (Object.prototype.hasOwnProperty.call(source, key)) {
          target[key] = source[key];
        }
      }
    }
    return target;
  };
  return _extends$1.apply(this, arguments);
}
const DataRouterContext = /* @__PURE__ */ reactExports.createContext(null);
if (false) {
  DataRouterContext.displayName = "DataRouter";
}
const DataRouterStateContext = /* @__PURE__ */ reactExports.createContext(null);
if (false) {
  DataRouterStateContext.displayName = "DataRouterState";
}
const AwaitContext = /* @__PURE__ */ reactExports.createContext(null);
if (false) {
  AwaitContext.displayName = "Await";
}
const NavigationContext = /* @__PURE__ */ reactExports.createContext(null);
if (false) {
  NavigationContext.displayName = "Navigation";
}
const LocationContext = /* @__PURE__ */ reactExports.createContext(null);
if (false) {
  LocationContext.displayName = "Location";
}
const RouteContext = /* @__PURE__ */ reactExports.createContext({
  outlet: null,
  matches: [],
  isDataRoute: false
});
if (false) {
  RouteContext.displayName = "Route";
}
const RouteErrorContext = /* @__PURE__ */ reactExports.createContext(null);
if (false) {
  RouteErrorContext.displayName = "RouteError";
}
function useHref(to, _temp) {
  let {
    relative
  } = _temp === void 0 ? {} : _temp;
  !useInRouterContext() ? false ? invariant(
    false,
    // TODO: This error is probably because they somehow have 2 versions of the
    // router loaded. We can help them understand how to avoid that.
    "useHref() may be used only in the context of a <Router> component."
  ) : invariant(false) : void 0;
  let {
    basename,
    navigator
  } = reactExports.useContext(NavigationContext);
  let {
    hash,
    pathname,
    search
  } = useResolvedPath(to, {
    relative
  });
  let joinedPathname = pathname;
  if (basename !== "/") {
    joinedPathname = pathname === "/" ? basename : joinPaths([basename, pathname]);
  }
  return navigator.createHref({
    pathname: joinedPathname,
    search,
    hash
  });
}
function useInRouterContext() {
  return reactExports.useContext(LocationContext) != null;
}
function useLocation() {
  !useInRouterContext() ? false ? invariant(
    false,
    // TODO: This error is probably because they somehow have 2 versions of the
    // router loaded. We can help them understand how to avoid that.
    "useLocation() may be used only in the context of a <Router> component."
  ) : invariant(false) : void 0;
  return reactExports.useContext(LocationContext).location;
}
function useNavigationType() {
  return reactExports.useContext(LocationContext).navigationType;
}
function useMatch(pattern) {
  !useInRouterContext() ? false ? invariant(
    false,
    // TODO: This error is probably because they somehow have 2 versions of the
    // router loaded. We can help them understand how to avoid that.
    "useMatch() may be used only in the context of a <Router> component."
  ) : invariant(false) : void 0;
  let {
    pathname
  } = useLocation();
  return reactExports.useMemo(() => matchPath(pattern, decodePath(pathname)), [pathname, pattern]);
}
const navigateEffectWarning = "You should call navigate() in a React.useEffect(), not when your component is first rendered.";
function useIsomorphicLayoutEffect(cb) {
  let isStatic = reactExports.useContext(NavigationContext).static;
  if (!isStatic) {
    reactExports.useLayoutEffect(cb);
  }
}
function useNavigate() {
  let {
    isDataRoute
  } = reactExports.useContext(RouteContext);
  return isDataRoute ? useNavigateStable() : useNavigateUnstable();
}
function useNavigateUnstable() {
  !useInRouterContext() ? false ? invariant(
    false,
    // TODO: This error is probably because they somehow have 2 versions of the
    // router loaded. We can help them understand how to avoid that.
    "useNavigate() may be used only in the context of a <Router> component."
  ) : invariant(false) : void 0;
  let dataRouterContext = reactExports.useContext(DataRouterContext);
  let {
    basename,
    future,
    navigator
  } = reactExports.useContext(NavigationContext);
  let {
    matches
  } = reactExports.useContext(RouteContext);
  let {
    pathname: locationPathname
  } = useLocation();
  let routePathnamesJson = JSON.stringify(getResolveToMatches(matches, future.v7_relativeSplatPath));
  let activeRef = reactExports.useRef(false);
  useIsomorphicLayoutEffect(() => {
    activeRef.current = true;
  });
  let navigate = reactExports.useCallback(function(to, options) {
    if (options === void 0) {
      options = {};
    }
    false ? warning(activeRef.current, navigateEffectWarning) : void 0;
    if (!activeRef.current) return;
    if (typeof to === "number") {
      navigator.go(to);
      return;
    }
    let path = resolveTo(to, JSON.parse(routePathnamesJson), locationPathname, options.relative === "path");
    if (dataRouterContext == null && basename !== "/") {
      path.pathname = path.pathname === "/" ? basename : joinPaths([basename, path.pathname]);
    }
    (!!options.replace ? navigator.replace : navigator.push)(path, options.state, options);
  }, [basename, navigator, routePathnamesJson, locationPathname, dataRouterContext]);
  return navigate;
}
const OutletContext = /* @__PURE__ */ reactExports.createContext(null);
function useOutletContext() {
  return reactExports.useContext(OutletContext);
}
function useOutlet(context) {
  let outlet = reactExports.useContext(RouteContext).outlet;
  if (outlet) {
    return /* @__PURE__ */ reactExports.createElement(OutletContext.Provider, {
      value: context
    }, outlet);
  }
  return outlet;
}
function useParams() {
  let {
    matches
  } = reactExports.useContext(RouteContext);
  let routeMatch = matches[matches.length - 1];
  return routeMatch ? routeMatch.params : {};
}
function useResolvedPath(to, _temp2) {
  let {
    relative
  } = _temp2 === void 0 ? {} : _temp2;
  let {
    future
  } = reactExports.useContext(NavigationContext);
  let {
    matches
  } = reactExports.useContext(RouteContext);
  let {
    pathname: locationPathname
  } = useLocation();
  let routePathnamesJson = JSON.stringify(getResolveToMatches(matches, future.v7_relativeSplatPath));
  return reactExports.useMemo(() => resolveTo(to, JSON.parse(routePathnamesJson), locationPathname, relative === "path"), [to, routePathnamesJson, locationPathname, relative]);
}
function useRoutes(routes, locationArg) {
  return useRoutesImpl(routes, locationArg);
}
function useRoutesImpl(routes, locationArg, dataRouterState, future) {
  !useInRouterContext() ? false ? invariant(
    false,
    // TODO: This error is probably because they somehow have 2 versions of the
    // router loaded. We can help them understand how to avoid that.
    "useRoutes() may be used only in the context of a <Router> component."
  ) : invariant(false) : void 0;
  let {
    navigator
  } = reactExports.useContext(NavigationContext);
  let {
    matches: parentMatches
  } = reactExports.useContext(RouteContext);
  let routeMatch = parentMatches[parentMatches.length - 1];
  let parentParams = routeMatch ? routeMatch.params : {};
  let parentPathname = routeMatch ? routeMatch.pathname : "/";
  let parentPathnameBase = routeMatch ? routeMatch.pathnameBase : "/";
  let parentRoute = routeMatch && routeMatch.route;
  if (false) {
    let parentPath = parentRoute && parentRoute.path || "";
    warningOnce(parentPathname, !parentRoute || parentPath.endsWith("*"), "You rendered descendant <Routes> (or called `useRoutes()`) at " + ('"' + parentPathname + '" (under <Route path="' + parentPath + '">) but the ') + `parent route path has no trailing "*". This means if you navigate deeper, the parent won't match anymore and therefore the child routes will never render.

` + ('Please change the parent <Route path="' + parentPath + '"> to <Route ') + ('path="' + (parentPath === "/" ? "*" : parentPath + "/*") + '">.'));
  }
  let locationFromContext = useLocation();
  let location;
  if (locationArg) {
    var _parsedLocationArg$pa;
    let parsedLocationArg = typeof locationArg === "string" ? parsePath(locationArg) : locationArg;
    !(parentPathnameBase === "/" || ((_parsedLocationArg$pa = parsedLocationArg.pathname) == null ? void 0 : _parsedLocationArg$pa.startsWith(parentPathnameBase))) ? false ? invariant(false, "When overriding the location using `<Routes location>` or `useRoutes(routes, location)`, the location pathname must begin with the portion of the URL pathname that was " + ('matched by all parent routes. The current pathname base is "' + parentPathnameBase + '" ') + ('but pathname "' + parsedLocationArg.pathname + '" was given in the `location` prop.')) : invariant(false) : void 0;
    location = parsedLocationArg;
  } else {
    location = locationFromContext;
  }
  let pathname = location.pathname || "/";
  let remainingPathname = pathname;
  if (parentPathnameBase !== "/") {
    let parentSegments = parentPathnameBase.replace(/^\//, "").split("/");
    let segments = pathname.replace(/^\//, "").split("/");
    remainingPathname = "/" + segments.slice(parentSegments.length).join("/");
  }
  let matches = matchRoutes(routes, {
    pathname: remainingPathname
  });
  if (false) {
    false ? warning(parentRoute || matches != null, 'No routes matched location "' + location.pathname + location.search + location.hash + '" ') : void 0;
    false ? warning(matches == null || matches[matches.length - 1].route.element !== void 0 || matches[matches.length - 1].route.Component !== void 0 || matches[matches.length - 1].route.lazy !== void 0, 'Matched leaf route at location "' + location.pathname + location.search + location.hash + '" does not have an element or Component. This means it will render an <Outlet /> with a null value by default resulting in an "empty" page.') : void 0;
  }
  let renderedMatches = _renderMatches(matches && matches.map((match) => Object.assign({}, match, {
    params: Object.assign({}, parentParams, match.params),
    pathname: joinPaths([
      parentPathnameBase,
      // Re-encode pathnames that were decoded inside matchRoutes
      navigator.encodeLocation ? navigator.encodeLocation(match.pathname).pathname : match.pathname
    ]),
    pathnameBase: match.pathnameBase === "/" ? parentPathnameBase : joinPaths([
      parentPathnameBase,
      // Re-encode pathnames that were decoded inside matchRoutes
      navigator.encodeLocation ? navigator.encodeLocation(match.pathnameBase).pathname : match.pathnameBase
    ])
  })), parentMatches, dataRouterState, future);
  if (locationArg && renderedMatches) {
    return /* @__PURE__ */ reactExports.createElement(LocationContext.Provider, {
      value: {
        location: _extends$1({
          pathname: "/",
          search: "",
          hash: "",
          state: null,
          key: "default"
        }, location),
        navigationType: Action.Pop
      }
    }, renderedMatches);
  }
  return renderedMatches;
}
function DefaultErrorComponent() {
  let error = useRouteError();
  let message = isRouteErrorResponse(error) ? error.status + " " + error.statusText : error instanceof Error ? error.message : JSON.stringify(error);
  let stack = error instanceof Error ? error.stack : null;
  let lightgrey = "rgba(200,200,200, 0.5)";
  let preStyles = {
    padding: "0.5rem",
    backgroundColor: lightgrey
  };
  let codeStyles = {
    padding: "2px 4px",
    backgroundColor: lightgrey
  };
  let devInfo = null;
  if (false) {
    console.error("Error handled by React Router default ErrorBoundary:", error);
    devInfo = /* @__PURE__ */ reactExports.createElement(reactExports.Fragment, null, /* @__PURE__ */ reactExports.createElement("p", null, "💿 Hey developer 👋"), /* @__PURE__ */ reactExports.createElement("p", null, "You can provide a way better UX than this when your app throws errors by providing your own ", /* @__PURE__ */ reactExports.createElement("code", {
      style: codeStyles
    }, "ErrorBoundary"), " or", " ", /* @__PURE__ */ reactExports.createElement("code", {
      style: codeStyles
    }, "errorElement"), " prop on your route."));
  }
  return /* @__PURE__ */ reactExports.createElement(reactExports.Fragment, null, /* @__PURE__ */ reactExports.createElement("h2", null, "Unexpected Application Error!"), /* @__PURE__ */ reactExports.createElement("h3", {
    style: {
      fontStyle: "italic"
    }
  }, message), stack ? /* @__PURE__ */ reactExports.createElement("pre", {
    style: preStyles
  }, stack) : null, devInfo);
}
const defaultErrorElement = /* @__PURE__ */ reactExports.createElement(DefaultErrorComponent, null);
class RenderErrorBoundary extends reactExports.Component {
  constructor(props) {
    super(props);
    this.state = {
      location: props.location,
      revalidation: props.revalidation,
      error: props.error
    };
  }
  static getDerivedStateFromError(error) {
    return {
      error
    };
  }
  static getDerivedStateFromProps(props, state) {
    if (state.location !== props.location || state.revalidation !== "idle" && props.revalidation === "idle") {
      return {
        error: props.error,
        location: props.location,
        revalidation: props.revalidation
      };
    }
    return {
      error: props.error !== void 0 ? props.error : state.error,
      location: state.location,
      revalidation: props.revalidation || state.revalidation
    };
  }
  componentDidCatch(error, errorInfo) {
    console.error("React Router caught the following error during render", error, errorInfo);
  }
  render() {
    return this.state.error !== void 0 ? /* @__PURE__ */ reactExports.createElement(RouteContext.Provider, {
      value: this.props.routeContext
    }, /* @__PURE__ */ reactExports.createElement(RouteErrorContext.Provider, {
      value: this.state.error,
      children: this.props.component
    })) : this.props.children;
  }
}
function RenderedRoute(_ref) {
  let {
    routeContext,
    match,
    children
  } = _ref;
  let dataRouterContext = reactExports.useContext(DataRouterContext);
  if (dataRouterContext && dataRouterContext.static && dataRouterContext.staticContext && (match.route.errorElement || match.route.ErrorBoundary)) {
    dataRouterContext.staticContext._deepestRenderedBoundaryId = match.route.id;
  }
  return /* @__PURE__ */ reactExports.createElement(RouteContext.Provider, {
    value: routeContext
  }, children);
}
function _renderMatches(matches, parentMatches, dataRouterState, future) {
  var _dataRouterState;
  if (parentMatches === void 0) {
    parentMatches = [];
  }
  if (dataRouterState === void 0) {
    dataRouterState = null;
  }
  if (future === void 0) {
    future = null;
  }
  if (matches == null) {
    var _future;
    if (!dataRouterState) {
      return null;
    }
    if (dataRouterState.errors) {
      matches = dataRouterState.matches;
    } else if ((_future = future) != null && _future.v7_partialHydration && parentMatches.length === 0 && !dataRouterState.initialized && dataRouterState.matches.length > 0) {
      matches = dataRouterState.matches;
    } else {
      return null;
    }
  }
  let renderedMatches = matches;
  let errors = (_dataRouterState = dataRouterState) == null ? void 0 : _dataRouterState.errors;
  if (errors != null) {
    let errorIndex = renderedMatches.findIndex((m) => m.route.id && (errors == null ? void 0 : errors[m.route.id]) !== void 0);
    !(errorIndex >= 0) ? false ? invariant(false, "Could not find a matching route for errors on route IDs: " + Object.keys(errors).join(",")) : invariant(false) : void 0;
    renderedMatches = renderedMatches.slice(0, Math.min(renderedMatches.length, errorIndex + 1));
  }
  let renderFallback = false;
  let fallbackIndex = -1;
  if (dataRouterState && future && future.v7_partialHydration) {
    for (let i = 0; i < renderedMatches.length; i++) {
      let match = renderedMatches[i];
      if (match.route.HydrateFallback || match.route.hydrateFallbackElement) {
        fallbackIndex = i;
      }
      if (match.route.id) {
        let {
          loaderData,
          errors: errors2
        } = dataRouterState;
        let needsToRunLoader = match.route.loader && loaderData[match.route.id] === void 0 && (!errors2 || errors2[match.route.id] === void 0);
        if (match.route.lazy || needsToRunLoader) {
          renderFallback = true;
          if (fallbackIndex >= 0) {
            renderedMatches = renderedMatches.slice(0, fallbackIndex + 1);
          } else {
            renderedMatches = [renderedMatches[0]];
          }
          break;
        }
      }
    }
  }
  return renderedMatches.reduceRight((outlet, match, index) => {
    let error;
    let shouldRenderHydrateFallback = false;
    let errorElement = null;
    let hydrateFallbackElement = null;
    if (dataRouterState) {
      error = errors && match.route.id ? errors[match.route.id] : void 0;
      errorElement = match.route.errorElement || defaultErrorElement;
      if (renderFallback) {
        if (fallbackIndex < 0 && index === 0) {
          warningOnce("route-fallback", false, "No `HydrateFallback` element provided to render during initial hydration");
          shouldRenderHydrateFallback = true;
          hydrateFallbackElement = null;
        } else if (fallbackIndex === index) {
          shouldRenderHydrateFallback = true;
          hydrateFallbackElement = match.route.hydrateFallbackElement || null;
        }
      }
    }
    let matches2 = parentMatches.concat(renderedMatches.slice(0, index + 1));
    let getChildren = () => {
      let children;
      if (error) {
        children = errorElement;
      } else if (shouldRenderHydrateFallback) {
        children = hydrateFallbackElement;
      } else if (match.route.Component) {
        children = /* @__PURE__ */ reactExports.createElement(match.route.Component, null);
      } else if (match.route.element) {
        children = match.route.element;
      } else {
        children = outlet;
      }
      return /* @__PURE__ */ reactExports.createElement(RenderedRoute, {
        match,
        routeContext: {
          outlet,
          matches: matches2,
          isDataRoute: dataRouterState != null
        },
        children
      });
    };
    return dataRouterState && (match.route.ErrorBoundary || match.route.errorElement || index === 0) ? /* @__PURE__ */ reactExports.createElement(RenderErrorBoundary, {
      location: dataRouterState.location,
      revalidation: dataRouterState.revalidation,
      component: errorElement,
      error,
      children: getChildren(),
      routeContext: {
        outlet: null,
        matches: matches2,
        isDataRoute: true
      }
    }) : getChildren();
  }, null);
}
var DataRouterHook$1 = /* @__PURE__ */ (function(DataRouterHook2) {
  DataRouterHook2["UseBlocker"] = "useBlocker";
  DataRouterHook2["UseRevalidator"] = "useRevalidator";
  DataRouterHook2["UseNavigateStable"] = "useNavigate";
  return DataRouterHook2;
})(DataRouterHook$1 || {});
var DataRouterStateHook$1 = /* @__PURE__ */ (function(DataRouterStateHook2) {
  DataRouterStateHook2["UseBlocker"] = "useBlocker";
  DataRouterStateHook2["UseLoaderData"] = "useLoaderData";
  DataRouterStateHook2["UseActionData"] = "useActionData";
  DataRouterStateHook2["UseRouteError"] = "useRouteError";
  DataRouterStateHook2["UseNavigation"] = "useNavigation";
  DataRouterStateHook2["UseRouteLoaderData"] = "useRouteLoaderData";
  DataRouterStateHook2["UseMatches"] = "useMatches";
  DataRouterStateHook2["UseRevalidator"] = "useRevalidator";
  DataRouterStateHook2["UseNavigateStable"] = "useNavigate";
  DataRouterStateHook2["UseRouteId"] = "useRouteId";
  return DataRouterStateHook2;
})(DataRouterStateHook$1 || {});
function getDataRouterConsoleError$1(hookName) {
  return hookName + " must be used within a data router.  See https://reactrouter.com/v6/routers/picking-a-router.";
}
function useDataRouterContext$1(hookName) {
  let ctx = reactExports.useContext(DataRouterContext);
  !ctx ? false ? invariant(false, getDataRouterConsoleError$1(hookName)) : invariant(false) : void 0;
  return ctx;
}
function useDataRouterState$1(hookName) {
  let state = reactExports.useContext(DataRouterStateContext);
  !state ? false ? invariant(false, getDataRouterConsoleError$1(hookName)) : invariant(false) : void 0;
  return state;
}
function useRouteContext(hookName) {
  let route = reactExports.useContext(RouteContext);
  !route ? false ? invariant(false, getDataRouterConsoleError$1(hookName)) : invariant(false) : void 0;
  return route;
}
function useCurrentRouteId(hookName) {
  let route = useRouteContext(hookName);
  let thisRoute = route.matches[route.matches.length - 1];
  !thisRoute.route.id ? false ? invariant(false, hookName + ' can only be used on routes that contain a unique "id"') : invariant(false) : void 0;
  return thisRoute.route.id;
}
function useRouteId() {
  return useCurrentRouteId(DataRouterStateHook$1.UseRouteId);
}
function useNavigation() {
  let state = useDataRouterState$1(DataRouterStateHook$1.UseNavigation);
  return state.navigation;
}
function useRevalidator() {
  let dataRouterContext = useDataRouterContext$1(DataRouterHook$1.UseRevalidator);
  let state = useDataRouterState$1(DataRouterStateHook$1.UseRevalidator);
  return reactExports.useMemo(() => ({
    revalidate: dataRouterContext.router.revalidate,
    state: state.revalidation
  }), [dataRouterContext.router.revalidate, state.revalidation]);
}
function useMatches() {
  let {
    matches,
    loaderData
  } = useDataRouterState$1(DataRouterStateHook$1.UseMatches);
  return reactExports.useMemo(() => matches.map((m) => convertRouteMatchToUiMatch(m, loaderData)), [matches, loaderData]);
}
function useLoaderData() {
  let state = useDataRouterState$1(DataRouterStateHook$1.UseLoaderData);
  let routeId = useCurrentRouteId(DataRouterStateHook$1.UseLoaderData);
  if (state.errors && state.errors[routeId] != null) {
    console.error("You cannot `useLoaderData` in an errorElement (routeId: " + routeId + ")");
    return void 0;
  }
  return state.loaderData[routeId];
}
function useRouteLoaderData(routeId) {
  let state = useDataRouterState$1(DataRouterStateHook$1.UseRouteLoaderData);
  return state.loaderData[routeId];
}
function useActionData() {
  let state = useDataRouterState$1(DataRouterStateHook$1.UseActionData);
  let routeId = useCurrentRouteId(DataRouterStateHook$1.UseLoaderData);
  return state.actionData ? state.actionData[routeId] : void 0;
}
function useRouteError() {
  var _state$errors;
  let error = reactExports.useContext(RouteErrorContext);
  let state = useDataRouterState$1(DataRouterStateHook$1.UseRouteError);
  let routeId = useCurrentRouteId(DataRouterStateHook$1.UseRouteError);
  if (error !== void 0) {
    return error;
  }
  return (_state$errors = state.errors) == null ? void 0 : _state$errors[routeId];
}
function useAsyncValue() {
  let value = reactExports.useContext(AwaitContext);
  return value == null ? void 0 : value._data;
}
function useAsyncError() {
  let value = reactExports.useContext(AwaitContext);
  return value == null ? void 0 : value._error;
}
let blockerId = 0;
function useBlocker(shouldBlock) {
  let {
    router,
    basename
  } = useDataRouterContext$1(DataRouterHook$1.UseBlocker);
  let state = useDataRouterState$1(DataRouterStateHook$1.UseBlocker);
  let [blockerKey, setBlockerKey] = reactExports.useState("");
  let blockerFunction = reactExports.useCallback((arg) => {
    if (typeof shouldBlock !== "function") {
      return !!shouldBlock;
    }
    if (basename === "/") {
      return shouldBlock(arg);
    }
    let {
      currentLocation,
      nextLocation,
      historyAction
    } = arg;
    return shouldBlock({
      currentLocation: _extends$1({}, currentLocation, {
        pathname: stripBasename(currentLocation.pathname, basename) || currentLocation.pathname
      }),
      nextLocation: _extends$1({}, nextLocation, {
        pathname: stripBasename(nextLocation.pathname, basename) || nextLocation.pathname
      }),
      historyAction
    });
  }, [basename, shouldBlock]);
  reactExports.useEffect(() => {
    let key = String(++blockerId);
    setBlockerKey(key);
    return () => router.deleteBlocker(key);
  }, [router]);
  reactExports.useEffect(() => {
    if (blockerKey !== "") {
      router.getBlocker(blockerKey, blockerFunction);
    }
  }, [router, blockerKey, blockerFunction]);
  return blockerKey && state.blockers.has(blockerKey) ? state.blockers.get(blockerKey) : IDLE_BLOCKER;
}
function useNavigateStable() {
  let {
    router
  } = useDataRouterContext$1(DataRouterHook$1.UseNavigateStable);
  let id = useCurrentRouteId(DataRouterStateHook$1.UseNavigateStable);
  let activeRef = reactExports.useRef(false);
  useIsomorphicLayoutEffect(() => {
    activeRef.current = true;
  });
  let navigate = reactExports.useCallback(function(to, options) {
    if (options === void 0) {
      options = {};
    }
    false ? warning(activeRef.current, navigateEffectWarning) : void 0;
    if (!activeRef.current) return;
    if (typeof to === "number") {
      router.navigate(to);
    } else {
      router.navigate(to, _extends$1({
        fromRouteId: id
      }, options));
    }
  }, [router, id]);
  return navigate;
}
const alreadyWarned$1 = {};
function warningOnce(key, cond, message) {
  if (!cond && !alreadyWarned$1[key]) {
    alreadyWarned$1[key] = true;
    false ? warning(false, message) : void 0;
  }
}
const alreadyWarned = {};
function warnOnce(key, message) {
  if (false) {
    alreadyWarned[message] = true;
    console.warn(message);
  }
}
const logDeprecation = (flag, msg, link) => warnOnce(flag, "⚠️ React Router Future Flag Warning: " + msg + ". " + ("You can use the `" + flag + "` future flag to opt-in early. ") + ("For more information, see " + link + "."));
function logV6DeprecationWarnings(renderFuture, routerFuture) {
  if ((renderFuture == null ? void 0 : renderFuture.v7_startTransition) === void 0) {
    logDeprecation("v7_startTransition", "React Router will begin wrapping state updates in `React.startTransition` in v7", "https://reactrouter.com/v6/upgrading/future#v7_starttransition");
  }
  if ((renderFuture == null ? void 0 : renderFuture.v7_relativeSplatPath) === void 0 && (!routerFuture || routerFuture.v7_relativeSplatPath === void 0)) {
    logDeprecation("v7_relativeSplatPath", "Relative route resolution within Splat routes is changing in v7", "https://reactrouter.com/v6/upgrading/future#v7_relativesplatpath");
  }
  if (routerFuture) {
    if (routerFuture.v7_fetcherPersist === void 0) {
      logDeprecation("v7_fetcherPersist", "The persistence behavior of fetchers is changing in v7", "https://reactrouter.com/v6/upgrading/future#v7_fetcherpersist");
    }
    if (routerFuture.v7_normalizeFormMethod === void 0) {
      logDeprecation("v7_normalizeFormMethod", "Casing of `formMethod` fields is being normalized to uppercase in v7", "https://reactrouter.com/v6/upgrading/future#v7_normalizeformmethod");
    }
    if (routerFuture.v7_partialHydration === void 0) {
      logDeprecation("v7_partialHydration", "`RouterProvider` hydration behavior is changing in v7", "https://reactrouter.com/v6/upgrading/future#v7_partialhydration");
    }
    if (routerFuture.v7_skipActionErrorRevalidation === void 0) {
      logDeprecation("v7_skipActionErrorRevalidation", "The revalidation behavior after 4xx/5xx `action` responses is changing in v7", "https://reactrouter.com/v6/upgrading/future#v7_skipactionerrorrevalidation");
    }
  }
}
const START_TRANSITION$1 = "startTransition";
const startTransitionImpl$1 = React$1[START_TRANSITION$1];
function RouterProvider$1(_ref) {
  let {
    fallbackElement,
    router,
    future
  } = _ref;
  let [state, setStateImpl] = reactExports.useState(router.state);
  let {
    v7_startTransition
  } = future || {};
  let setState = reactExports.useCallback((newState) => {
    if (v7_startTransition && startTransitionImpl$1) {
      startTransitionImpl$1(() => setStateImpl(newState));
    } else {
      setStateImpl(newState);
    }
  }, [setStateImpl, v7_startTransition]);
  reactExports.useLayoutEffect(() => router.subscribe(setState), [router, setState]);
  reactExports.useEffect(() => {
    false ? warning(fallbackElement == null || !router.future.v7_partialHydration, "`<RouterProvider fallbackElement>` is deprecated when using `v7_partialHydration`, use a `HydrateFallback` component instead") : void 0;
  }, []);
  let navigator = reactExports.useMemo(() => {
    return {
      createHref: router.createHref,
      encodeLocation: router.encodeLocation,
      go: (n) => router.navigate(n),
      push: (to, state2, opts) => router.navigate(to, {
        state: state2,
        preventScrollReset: opts == null ? void 0 : opts.preventScrollReset
      }),
      replace: (to, state2, opts) => router.navigate(to, {
        replace: true,
        state: state2,
        preventScrollReset: opts == null ? void 0 : opts.preventScrollReset
      })
    };
  }, [router]);
  let basename = router.basename || "/";
  let dataRouterContext = reactExports.useMemo(() => ({
    router,
    navigator,
    static: false,
    basename
  }), [router, navigator, basename]);
  reactExports.useEffect(() => logV6DeprecationWarnings(future, router.future), [router, future]);
  return /* @__PURE__ */ reactExports.createElement(reactExports.Fragment, null, /* @__PURE__ */ reactExports.createElement(DataRouterContext.Provider, {
    value: dataRouterContext
  }, /* @__PURE__ */ reactExports.createElement(DataRouterStateContext.Provider, {
    value: state
  }, /* @__PURE__ */ reactExports.createElement(Router, {
    basename,
    location: state.location,
    navigationType: state.historyAction,
    navigator,
    future: {
      v7_relativeSplatPath: router.future.v7_relativeSplatPath
    }
  }, state.initialized || router.future.v7_partialHydration ? /* @__PURE__ */ reactExports.createElement(DataRoutes$1, {
    routes: router.routes,
    future: router.future,
    state
  }) : fallbackElement))), null);
}
function DataRoutes$1(_ref2) {
  let {
    routes,
    future,
    state
  } = _ref2;
  return useRoutesImpl(routes, void 0, state, future);
}
function MemoryRouter(_ref3) {
  let {
    basename,
    children,
    initialEntries,
    initialIndex,
    future
  } = _ref3;
  let historyRef = reactExports.useRef();
  if (historyRef.current == null) {
    historyRef.current = createMemoryHistory({
      initialEntries,
      initialIndex,
      v5Compat: true
    });
  }
  let history = historyRef.current;
  let [state, setStateImpl] = reactExports.useState({
    action: history.action,
    location: history.location
  });
  let {
    v7_startTransition
  } = future || {};
  let setState = reactExports.useCallback((newState) => {
    v7_startTransition && startTransitionImpl$1 ? startTransitionImpl$1(() => setStateImpl(newState)) : setStateImpl(newState);
  }, [setStateImpl, v7_startTransition]);
  reactExports.useLayoutEffect(() => history.listen(setState), [history, setState]);
  reactExports.useEffect(() => logV6DeprecationWarnings(future), [future]);
  return /* @__PURE__ */ reactExports.createElement(Router, {
    basename,
    children,
    location: state.location,
    navigationType: state.action,
    navigator: history,
    future
  });
}
function Navigate(_ref4) {
  let {
    to,
    replace: replace2,
    state,
    relative
  } = _ref4;
  !useInRouterContext() ? false ? invariant(
    false,
    // TODO: This error is probably because they somehow have 2 versions of
    // the router loaded. We can help them understand how to avoid that.
    "<Navigate> may be used only in the context of a <Router> component."
  ) : invariant(false) : void 0;
  let {
    future,
    static: isStatic
  } = reactExports.useContext(NavigationContext);
  false ? warning(!isStatic, "<Navigate> must not be used on the initial render in a <StaticRouter>. This is a no-op, but you should modify your code so the <Navigate> is only ever rendered in response to some user interaction or state change.") : void 0;
  let {
    matches
  } = reactExports.useContext(RouteContext);
  let {
    pathname: locationPathname
  } = useLocation();
  let navigate = useNavigate();
  let path = resolveTo(to, getResolveToMatches(matches, future.v7_relativeSplatPath), locationPathname, relative === "path");
  let jsonPath = JSON.stringify(path);
  reactExports.useEffect(() => navigate(JSON.parse(jsonPath), {
    replace: replace2,
    state,
    relative
  }), [navigate, jsonPath, relative, replace2, state]);
  return null;
}
function Outlet(props) {
  return useOutlet(props.context);
}
function Route(_props) {
  false ? invariant(false, "A <Route> is only ever to be used as the child of <Routes> element, never rendered directly. Please wrap your <Route> in a <Routes>.") : invariant(false);
}
function Router(_ref5) {
  let {
    basename: basenameProp = "/",
    children = null,
    location: locationProp,
    navigationType = Action.Pop,
    navigator,
    static: staticProp = false,
    future
  } = _ref5;
  !!useInRouterContext() ? false ? invariant(false, "You cannot render a <Router> inside another <Router>. You should never have more than one in your app.") : invariant(false) : void 0;
  let basename = basenameProp.replace(/^\/*/, "/");
  let navigationContext = reactExports.useMemo(() => ({
    basename,
    navigator,
    static: staticProp,
    future: _extends$1({
      v7_relativeSplatPath: false
    }, future)
  }), [basename, future, navigator, staticProp]);
  if (typeof locationProp === "string") {
    locationProp = parsePath(locationProp);
  }
  let {
    pathname = "/",
    search = "",
    hash = "",
    state = null,
    key = "default"
  } = locationProp;
  let locationContext = reactExports.useMemo(() => {
    let trailingPathname = stripBasename(pathname, basename);
    if (trailingPathname == null) {
      return null;
    }
    return {
      location: {
        pathname: trailingPathname,
        search,
        hash,
        state,
        key
      },
      navigationType
    };
  }, [basename, pathname, search, hash, state, key, navigationType]);
  false ? warning(locationContext != null, '<Router basename="' + basename + '"> is not able to match the URL ' + ('"' + pathname + search + hash + '" because it does not start with the ') + "basename, so the <Router> won't render anything.") : void 0;
  if (locationContext == null) {
    return null;
  }
  return /* @__PURE__ */ reactExports.createElement(NavigationContext.Provider, {
    value: navigationContext
  }, /* @__PURE__ */ reactExports.createElement(LocationContext.Provider, {
    children,
    value: locationContext
  }));
}
function Routes(_ref6) {
  let {
    children,
    location
  } = _ref6;
  return useRoutes(createRoutesFromChildren(children), location);
}
function Await(_ref7) {
  let {
    children,
    errorElement,
    resolve
  } = _ref7;
  return /* @__PURE__ */ reactExports.createElement(AwaitErrorBoundary, {
    resolve,
    errorElement
  }, /* @__PURE__ */ reactExports.createElement(ResolveAwait, null, children));
}
var AwaitRenderStatus = /* @__PURE__ */ (function(AwaitRenderStatus2) {
  AwaitRenderStatus2[AwaitRenderStatus2["pending"] = 0] = "pending";
  AwaitRenderStatus2[AwaitRenderStatus2["success"] = 1] = "success";
  AwaitRenderStatus2[AwaitRenderStatus2["error"] = 2] = "error";
  return AwaitRenderStatus2;
})(AwaitRenderStatus || {});
const neverSettledPromise = new Promise(() => {
});
class AwaitErrorBoundary extends reactExports.Component {
  constructor(props) {
    super(props);
    this.state = {
      error: null
    };
  }
  static getDerivedStateFromError(error) {
    return {
      error
    };
  }
  componentDidCatch(error, errorInfo) {
    console.error("<Await> caught the following error during render", error, errorInfo);
  }
  render() {
    let {
      children,
      errorElement,
      resolve
    } = this.props;
    let promise = null;
    let status = AwaitRenderStatus.pending;
    if (!(resolve instanceof Promise)) {
      status = AwaitRenderStatus.success;
      promise = Promise.resolve();
      Object.defineProperty(promise, "_tracked", {
        get: () => true
      });
      Object.defineProperty(promise, "_data", {
        get: () => resolve
      });
    } else if (this.state.error) {
      status = AwaitRenderStatus.error;
      let renderError = this.state.error;
      promise = Promise.reject().catch(() => {
      });
      Object.defineProperty(promise, "_tracked", {
        get: () => true
      });
      Object.defineProperty(promise, "_error", {
        get: () => renderError
      });
    } else if (resolve._tracked) {
      promise = resolve;
      status = "_error" in promise ? AwaitRenderStatus.error : "_data" in promise ? AwaitRenderStatus.success : AwaitRenderStatus.pending;
    } else {
      status = AwaitRenderStatus.pending;
      Object.defineProperty(resolve, "_tracked", {
        get: () => true
      });
      promise = resolve.then((data) => Object.defineProperty(resolve, "_data", {
        get: () => data
      }), (error) => Object.defineProperty(resolve, "_error", {
        get: () => error
      }));
    }
    if (status === AwaitRenderStatus.error && promise._error instanceof AbortedDeferredError) {
      throw neverSettledPromise;
    }
    if (status === AwaitRenderStatus.error && !errorElement) {
      throw promise._error;
    }
    if (status === AwaitRenderStatus.error) {
      return /* @__PURE__ */ reactExports.createElement(AwaitContext.Provider, {
        value: promise,
        children: errorElement
      });
    }
    if (status === AwaitRenderStatus.success) {
      return /* @__PURE__ */ reactExports.createElement(AwaitContext.Provider, {
        value: promise,
        children
      });
    }
    throw promise;
  }
}
function ResolveAwait(_ref8) {
  let {
    children
  } = _ref8;
  let data = useAsyncValue();
  let toRender = typeof children === "function" ? children(data) : children;
  return /* @__PURE__ */ reactExports.createElement(reactExports.Fragment, null, toRender);
}
function createRoutesFromChildren(children, parentPath) {
  if (parentPath === void 0) {
    parentPath = [];
  }
  let routes = [];
  reactExports.Children.forEach(children, (element, index) => {
    if (!/* @__PURE__ */ reactExports.isValidElement(element)) {
      return;
    }
    let treePath = [...parentPath, index];
    if (element.type === reactExports.Fragment) {
      routes.push.apply(routes, createRoutesFromChildren(element.props.children, treePath));
      return;
    }
    !(element.type === Route) ? false ? invariant(false, "[" + (typeof element.type === "string" ? element.type : element.type.name) + "] is not a <Route> component. All component children of <Routes> must be a <Route> or <React.Fragment>") : invariant(false) : void 0;
    !(!element.props.index || !element.props.children) ? false ? invariant(false, "An index route cannot have child routes.") : invariant(false) : void 0;
    let route = {
      id: element.props.id || treePath.join("-"),
      caseSensitive: element.props.caseSensitive,
      element: element.props.element,
      Component: element.props.Component,
      index: element.props.index,
      path: element.props.path,
      loader: element.props.loader,
      action: element.props.action,
      errorElement: element.props.errorElement,
      ErrorBoundary: element.props.ErrorBoundary,
      hasErrorBoundary: element.props.ErrorBoundary != null || element.props.errorElement != null,
      shouldRevalidate: element.props.shouldRevalidate,
      handle: element.props.handle,
      lazy: element.props.lazy
    };
    if (element.props.children) {
      route.children = createRoutesFromChildren(element.props.children, treePath);
    }
    routes.push(route);
  });
  return routes;
}
function renderMatches(matches) {
  return _renderMatches(matches);
}
function mapRouteProperties(route) {
  let updates = {
    // Note: this check also occurs in createRoutesFromChildren so update
    // there if you change this -- please and thank you!
    hasErrorBoundary: route.ErrorBoundary != null || route.errorElement != null
  };
  if (route.Component) {
    if (false) {
      if (route.element) {
        false ? warning(false, "You should not include both `Component` and `element` on your route - `Component` will be used.") : void 0;
      }
    }
    Object.assign(updates, {
      element: /* @__PURE__ */ reactExports.createElement(route.Component),
      Component: void 0
    });
  }
  if (route.HydrateFallback) {
    if (false) {
      if (route.hydrateFallbackElement) {
        false ? warning(false, "You should not include both `HydrateFallback` and `hydrateFallbackElement` on your route - `HydrateFallback` will be used.") : void 0;
      }
    }
    Object.assign(updates, {
      hydrateFallbackElement: /* @__PURE__ */ reactExports.createElement(route.HydrateFallback),
      HydrateFallback: void 0
    });
  }
  if (route.ErrorBoundary) {
    if (false) {
      if (route.errorElement) {
        false ? warning(false, "You should not include both `ErrorBoundary` and `errorElement` on your route - `ErrorBoundary` will be used.") : void 0;
      }
    }
    Object.assign(updates, {
      errorElement: /* @__PURE__ */ reactExports.createElement(route.ErrorBoundary),
      ErrorBoundary: void 0
    });
  }
  return updates;
}
function createMemoryRouter(routes, opts) {
  return createRouter({
    basename: opts == null ? void 0 : opts.basename,
    future: _extends$1({}, opts == null ? void 0 : opts.future, {
      v7_prependBasename: true
    }),
    history: createMemoryHistory({
      initialEntries: opts == null ? void 0 : opts.initialEntries,
      initialIndex: opts == null ? void 0 : opts.initialIndex
    }),
    hydrationData: opts == null ? void 0 : opts.hydrationData,
    routes,
    mapRouteProperties,
    dataStrategy: opts == null ? void 0 : opts.dataStrategy,
    patchRoutesOnNavigation: opts == null ? void 0 : opts.patchRoutesOnNavigation
  }).initialize();
}

/**
 * React Router DOM v6.30.2
 *
 * Copyright (c) Remix Software Inc.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE.md file in the root directory of this source tree.
 *
 * @license MIT
 */
function _extends() {
  _extends = Object.assign ? Object.assign.bind() : function(target) {
    for (var i = 1; i < arguments.length; i++) {
      var source = arguments[i];
      for (var key in source) {
        if (Object.prototype.hasOwnProperty.call(source, key)) {
          target[key] = source[key];
        }
      }
    }
    return target;
  };
  return _extends.apply(this, arguments);
}
function _objectWithoutPropertiesLoose(source, excluded) {
  if (source == null) return {};
  var target = {};
  var sourceKeys = Object.keys(source);
  var key, i;
  for (i = 0; i < sourceKeys.length; i++) {
    key = sourceKeys[i];
    if (excluded.indexOf(key) >= 0) continue;
    target[key] = source[key];
  }
  return target;
}
const defaultMethod = "get";
const defaultEncType = "application/x-www-form-urlencoded";
function isHtmlElement(object) {
  return object != null && typeof object.tagName === "string";
}
function isButtonElement(object) {
  return isHtmlElement(object) && object.tagName.toLowerCase() === "button";
}
function isFormElement(object) {
  return isHtmlElement(object) && object.tagName.toLowerCase() === "form";
}
function isInputElement(object) {
  return isHtmlElement(object) && object.tagName.toLowerCase() === "input";
}
function isModifiedEvent(event) {
  return !!(event.metaKey || event.altKey || event.ctrlKey || event.shiftKey);
}
function shouldProcessLinkClick(event, target) {
  return event.button === 0 && // Ignore everything but left clicks
  (!target || target === "_self") && // Let browser handle "target=_blank" etc.
  !isModifiedEvent(event);
}
function createSearchParams(init) {
  if (init === void 0) {
    init = "";
  }
  return new URLSearchParams(typeof init === "string" || Array.isArray(init) || init instanceof URLSearchParams ? init : Object.keys(init).reduce((memo, key) => {
    let value = init[key];
    return memo.concat(Array.isArray(value) ? value.map((v) => [key, v]) : [[key, value]]);
  }, []));
}
function getSearchParamsForLocation(locationSearch, defaultSearchParams) {
  let searchParams = createSearchParams(locationSearch);
  if (defaultSearchParams) {
    defaultSearchParams.forEach((_, key) => {
      if (!searchParams.has(key)) {
        defaultSearchParams.getAll(key).forEach((value) => {
          searchParams.append(key, value);
        });
      }
    });
  }
  return searchParams;
}
let _formDataSupportsSubmitter = null;
function isFormDataSubmitterSupported() {
  if (_formDataSupportsSubmitter === null) {
    try {
      new FormData(
        document.createElement("form"),
        // @ts-expect-error if FormData supports the submitter parameter, this will throw
        0
      );
      _formDataSupportsSubmitter = false;
    } catch (e) {
      _formDataSupportsSubmitter = true;
    }
  }
  return _formDataSupportsSubmitter;
}
const supportedFormEncTypes = /* @__PURE__ */ new Set(["application/x-www-form-urlencoded", "multipart/form-data", "text/plain"]);
function getFormEncType(encType) {
  if (encType != null && !supportedFormEncTypes.has(encType)) {
    false ? warning(false, '"' + encType + '" is not a valid `encType` for `<Form>`/`<fetcher.Form>` ' + ('and will default to "' + defaultEncType + '"')) : void 0;
    return null;
  }
  return encType;
}
function getFormSubmissionInfo(target, basename) {
  let method;
  let action;
  let encType;
  let formData;
  let body;
  if (isFormElement(target)) {
    let attr = target.getAttribute("action");
    action = attr ? stripBasename(attr, basename) : null;
    method = target.getAttribute("method") || defaultMethod;
    encType = getFormEncType(target.getAttribute("enctype")) || defaultEncType;
    formData = new FormData(target);
  } else if (isButtonElement(target) || isInputElement(target) && (target.type === "submit" || target.type === "image")) {
    let form = target.form;
    if (form == null) {
      throw new Error('Cannot submit a <button> or <input type="submit"> without a <form>');
    }
    let attr = target.getAttribute("formaction") || form.getAttribute("action");
    action = attr ? stripBasename(attr, basename) : null;
    method = target.getAttribute("formmethod") || form.getAttribute("method") || defaultMethod;
    encType = getFormEncType(target.getAttribute("formenctype")) || getFormEncType(form.getAttribute("enctype")) || defaultEncType;
    formData = new FormData(form, target);
    if (!isFormDataSubmitterSupported()) {
      let {
        name,
        type,
        value
      } = target;
      if (type === "image") {
        let prefix = name ? name + "." : "";
        formData.append(prefix + "x", "0");
        formData.append(prefix + "y", "0");
      } else if (name) {
        formData.append(name, value);
      }
    }
  } else if (isHtmlElement(target)) {
    throw new Error('Cannot submit element that is not <form>, <button>, or <input type="submit|image">');
  } else {
    method = defaultMethod;
    action = null;
    encType = defaultEncType;
    body = target;
  }
  if (formData && encType === "text/plain") {
    body = formData;
    formData = void 0;
  }
  return {
    action,
    method: method.toLowerCase(),
    encType,
    formData,
    body
  };
}
const _excluded = ["onClick", "relative", "reloadDocument", "replace", "state", "target", "to", "preventScrollReset", "viewTransition"], _excluded2 = ["aria-current", "caseSensitive", "className", "end", "style", "to", "viewTransition", "children"], _excluded3 = ["fetcherKey", "navigate", "reloadDocument", "replace", "state", "method", "action", "onSubmit", "relative", "preventScrollReset", "viewTransition"];
const REACT_ROUTER_VERSION = "6";
try {
  window.__reactRouterVersion = REACT_ROUTER_VERSION;
} catch (e) {
}
function createBrowserRouter(routes, opts) {
  return createRouter({
    basename: opts == null ? void 0 : opts.basename,
    future: _extends({}, opts == null ? void 0 : opts.future, {
      v7_prependBasename: true
    }),
    history: createBrowserHistory({
      window: opts == null ? void 0 : opts.window
    }),
    hydrationData: (opts == null ? void 0 : opts.hydrationData) || parseHydrationData(),
    routes,
    mapRouteProperties: mapRouteProperties,
    dataStrategy: opts == null ? void 0 : opts.dataStrategy,
    patchRoutesOnNavigation: opts == null ? void 0 : opts.patchRoutesOnNavigation,
    window: opts == null ? void 0 : opts.window
  }).initialize();
}
function createHashRouter(routes, opts) {
  return createRouter({
    basename: opts == null ? void 0 : opts.basename,
    future: _extends({}, opts == null ? void 0 : opts.future, {
      v7_prependBasename: true
    }),
    history: createHashHistory({
      window: opts == null ? void 0 : opts.window
    }),
    hydrationData: (opts == null ? void 0 : opts.hydrationData) || parseHydrationData(),
    routes,
    mapRouteProperties: mapRouteProperties,
    dataStrategy: opts == null ? void 0 : opts.dataStrategy,
    patchRoutesOnNavigation: opts == null ? void 0 : opts.patchRoutesOnNavigation,
    window: opts == null ? void 0 : opts.window
  }).initialize();
}
function parseHydrationData() {
  var _window;
  let state = (_window = window) == null ? void 0 : _window.__staticRouterHydrationData;
  if (state && state.errors) {
    state = _extends({}, state, {
      errors: deserializeErrors(state.errors)
    });
  }
  return state;
}
function deserializeErrors(errors) {
  if (!errors) return null;
  let entries = Object.entries(errors);
  let serialized = {};
  for (let [key, val] of entries) {
    if (val && val.__type === "RouteErrorResponse") {
      serialized[key] = new ErrorResponseImpl(val.status, val.statusText, val.data, val.internal === true);
    } else if (val && val.__type === "Error") {
      if (val.__subType) {
        let ErrorConstructor = window[val.__subType];
        if (typeof ErrorConstructor === "function") {
          try {
            let error = new ErrorConstructor(val.message);
            error.stack = "";
            serialized[key] = error;
          } catch (e) {
          }
        }
      }
      if (serialized[key] == null) {
        let error = new Error(val.message);
        error.stack = "";
        serialized[key] = error;
      }
    } else {
      serialized[key] = val;
    }
  }
  return serialized;
}
const ViewTransitionContext = /* @__PURE__ */ reactExports.createContext({
  isTransitioning: false
});
if (false) {
  ViewTransitionContext.displayName = "ViewTransition";
}
const FetchersContext = /* @__PURE__ */ reactExports.createContext(/* @__PURE__ */ new Map());
if (false) {
  FetchersContext.displayName = "Fetchers";
}
const START_TRANSITION = "startTransition";
const startTransitionImpl = React$1[START_TRANSITION];
const FLUSH_SYNC = "flushSync";
const flushSyncImpl = ReactDOM$1[FLUSH_SYNC];
const USE_ID = "useId";
const useIdImpl = React$1[USE_ID];
function startTransitionSafe(cb) {
  if (startTransitionImpl) {
    startTransitionImpl(cb);
  } else {
    cb();
  }
}
function flushSyncSafe(cb) {
  if (flushSyncImpl) {
    flushSyncImpl(cb);
  } else {
    cb();
  }
}
class Deferred {
  constructor() {
    this.status = "pending";
    this.promise = new Promise((resolve, reject) => {
      this.resolve = (value) => {
        if (this.status === "pending") {
          this.status = "resolved";
          resolve(value);
        }
      };
      this.reject = (reason) => {
        if (this.status === "pending") {
          this.status = "rejected";
          reject(reason);
        }
      };
    });
  }
}
function RouterProvider(_ref) {
  let {
    fallbackElement,
    router,
    future
  } = _ref;
  let [state, setStateImpl] = reactExports.useState(router.state);
  let [pendingState, setPendingState] = reactExports.useState();
  let [vtContext, setVtContext] = reactExports.useState({
    isTransitioning: false
  });
  let [renderDfd, setRenderDfd] = reactExports.useState();
  let [transition, setTransition] = reactExports.useState();
  let [interruption, setInterruption] = reactExports.useState();
  let fetcherData = reactExports.useRef(/* @__PURE__ */ new Map());
  let {
    v7_startTransition
  } = future || {};
  let optInStartTransition = reactExports.useCallback((cb) => {
    if (v7_startTransition) {
      startTransitionSafe(cb);
    } else {
      cb();
    }
  }, [v7_startTransition]);
  let setState = reactExports.useCallback((newState, _ref2) => {
    let {
      deletedFetchers,
      flushSync,
      viewTransitionOpts
    } = _ref2;
    newState.fetchers.forEach((fetcher, key) => {
      if (fetcher.data !== void 0) {
        fetcherData.current.set(key, fetcher.data);
      }
    });
    deletedFetchers.forEach((key) => fetcherData.current.delete(key));
    let isViewTransitionUnavailable = router.window == null || router.window.document == null || typeof router.window.document.startViewTransition !== "function";
    if (!viewTransitionOpts || isViewTransitionUnavailable) {
      if (flushSync) {
        flushSyncSafe(() => setStateImpl(newState));
      } else {
        optInStartTransition(() => setStateImpl(newState));
      }
      return;
    }
    if (flushSync) {
      flushSyncSafe(() => {
        if (transition) {
          renderDfd && renderDfd.resolve();
          transition.skipTransition();
        }
        setVtContext({
          isTransitioning: true,
          flushSync: true,
          currentLocation: viewTransitionOpts.currentLocation,
          nextLocation: viewTransitionOpts.nextLocation
        });
      });
      let t = router.window.document.startViewTransition(() => {
        flushSyncSafe(() => setStateImpl(newState));
      });
      t.finished.finally(() => {
        flushSyncSafe(() => {
          setRenderDfd(void 0);
          setTransition(void 0);
          setPendingState(void 0);
          setVtContext({
            isTransitioning: false
          });
        });
      });
      flushSyncSafe(() => setTransition(t));
      return;
    }
    if (transition) {
      renderDfd && renderDfd.resolve();
      transition.skipTransition();
      setInterruption({
        state: newState,
        currentLocation: viewTransitionOpts.currentLocation,
        nextLocation: viewTransitionOpts.nextLocation
      });
    } else {
      setPendingState(newState);
      setVtContext({
        isTransitioning: true,
        flushSync: false,
        currentLocation: viewTransitionOpts.currentLocation,
        nextLocation: viewTransitionOpts.nextLocation
      });
    }
  }, [router.window, transition, renderDfd, fetcherData, optInStartTransition]);
  reactExports.useLayoutEffect(() => router.subscribe(setState), [router, setState]);
  reactExports.useEffect(() => {
    if (vtContext.isTransitioning && !vtContext.flushSync) {
      setRenderDfd(new Deferred());
    }
  }, [vtContext]);
  reactExports.useEffect(() => {
    if (renderDfd && pendingState && router.window) {
      let newState = pendingState;
      let renderPromise = renderDfd.promise;
      let transition2 = router.window.document.startViewTransition(async () => {
        optInStartTransition(() => setStateImpl(newState));
        await renderPromise;
      });
      transition2.finished.finally(() => {
        setRenderDfd(void 0);
        setTransition(void 0);
        setPendingState(void 0);
        setVtContext({
          isTransitioning: false
        });
      });
      setTransition(transition2);
    }
  }, [optInStartTransition, pendingState, renderDfd, router.window]);
  reactExports.useEffect(() => {
    if (renderDfd && pendingState && state.location.key === pendingState.location.key) {
      renderDfd.resolve();
    }
  }, [renderDfd, transition, state.location, pendingState]);
  reactExports.useEffect(() => {
    if (!vtContext.isTransitioning && interruption) {
      setPendingState(interruption.state);
      setVtContext({
        isTransitioning: true,
        flushSync: false,
        currentLocation: interruption.currentLocation,
        nextLocation: interruption.nextLocation
      });
      setInterruption(void 0);
    }
  }, [vtContext.isTransitioning, interruption]);
  reactExports.useEffect(() => {
    false ? warning(fallbackElement == null || !router.future.v7_partialHydration, "`<RouterProvider fallbackElement>` is deprecated when using `v7_partialHydration`, use a `HydrateFallback` component instead") : void 0;
  }, []);
  let navigator = reactExports.useMemo(() => {
    return {
      createHref: router.createHref,
      encodeLocation: router.encodeLocation,
      go: (n) => router.navigate(n),
      push: (to, state2, opts) => router.navigate(to, {
        state: state2,
        preventScrollReset: opts == null ? void 0 : opts.preventScrollReset
      }),
      replace: (to, state2, opts) => router.navigate(to, {
        replace: true,
        state: state2,
        preventScrollReset: opts == null ? void 0 : opts.preventScrollReset
      })
    };
  }, [router]);
  let basename = router.basename || "/";
  let dataRouterContext = reactExports.useMemo(() => ({
    router,
    navigator,
    static: false,
    basename
  }), [router, navigator, basename]);
  let routerFuture = reactExports.useMemo(() => ({
    v7_relativeSplatPath: router.future.v7_relativeSplatPath
  }), [router.future.v7_relativeSplatPath]);
  reactExports.useEffect(() => logV6DeprecationWarnings(future, router.future), [future, router.future]);
  return /* @__PURE__ */ reactExports.createElement(reactExports.Fragment, null, /* @__PURE__ */ reactExports.createElement(DataRouterContext.Provider, {
    value: dataRouterContext
  }, /* @__PURE__ */ reactExports.createElement(DataRouterStateContext.Provider, {
    value: state
  }, /* @__PURE__ */ reactExports.createElement(FetchersContext.Provider, {
    value: fetcherData.current
  }, /* @__PURE__ */ reactExports.createElement(ViewTransitionContext.Provider, {
    value: vtContext
  }, /* @__PURE__ */ reactExports.createElement(Router, {
    basename,
    location: state.location,
    navigationType: state.historyAction,
    navigator,
    future: routerFuture
  }, state.initialized || router.future.v7_partialHydration ? /* @__PURE__ */ reactExports.createElement(MemoizedDataRoutes, {
    routes: router.routes,
    future: router.future,
    state
  }) : fallbackElement))))), null);
}
const MemoizedDataRoutes = /* @__PURE__ */ reactExports.memo(DataRoutes);
function DataRoutes(_ref3) {
  let {
    routes,
    future,
    state
  } = _ref3;
  return useRoutesImpl(routes, void 0, state, future);
}
function BrowserRouter(_ref4) {
  let {
    basename,
    children,
    future,
    window: window2
  } = _ref4;
  let historyRef = reactExports.useRef();
  if (historyRef.current == null) {
    historyRef.current = createBrowserHistory({
      window: window2,
      v5Compat: true
    });
  }
  let history = historyRef.current;
  let [state, setStateImpl] = reactExports.useState({
    action: history.action,
    location: history.location
  });
  let {
    v7_startTransition
  } = future || {};
  let setState = reactExports.useCallback((newState) => {
    v7_startTransition && startTransitionImpl ? startTransitionImpl(() => setStateImpl(newState)) : setStateImpl(newState);
  }, [setStateImpl, v7_startTransition]);
  reactExports.useLayoutEffect(() => history.listen(setState), [history, setState]);
  reactExports.useEffect(() => logV6DeprecationWarnings(future), [future]);
  return /* @__PURE__ */ reactExports.createElement(Router, {
    basename,
    children,
    location: state.location,
    navigationType: state.action,
    navigator: history,
    future
  });
}
function HashRouter(_ref5) {
  let {
    basename,
    children,
    future,
    window: window2
  } = _ref5;
  let historyRef = reactExports.useRef();
  if (historyRef.current == null) {
    historyRef.current = createHashHistory({
      window: window2,
      v5Compat: true
    });
  }
  let history = historyRef.current;
  let [state, setStateImpl] = reactExports.useState({
    action: history.action,
    location: history.location
  });
  let {
    v7_startTransition
  } = future || {};
  let setState = reactExports.useCallback((newState) => {
    v7_startTransition && startTransitionImpl ? startTransitionImpl(() => setStateImpl(newState)) : setStateImpl(newState);
  }, [setStateImpl, v7_startTransition]);
  reactExports.useLayoutEffect(() => history.listen(setState), [history, setState]);
  reactExports.useEffect(() => logV6DeprecationWarnings(future), [future]);
  return /* @__PURE__ */ reactExports.createElement(Router, {
    basename,
    children,
    location: state.location,
    navigationType: state.action,
    navigator: history,
    future
  });
}
function HistoryRouter(_ref6) {
  let {
    basename,
    children,
    future,
    history
  } = _ref6;
  let [state, setStateImpl] = reactExports.useState({
    action: history.action,
    location: history.location
  });
  let {
    v7_startTransition
  } = future || {};
  let setState = reactExports.useCallback((newState) => {
    v7_startTransition && startTransitionImpl ? startTransitionImpl(() => setStateImpl(newState)) : setStateImpl(newState);
  }, [setStateImpl, v7_startTransition]);
  reactExports.useLayoutEffect(() => history.listen(setState), [history, setState]);
  reactExports.useEffect(() => logV6DeprecationWarnings(future), [future]);
  return /* @__PURE__ */ reactExports.createElement(Router, {
    basename,
    children,
    location: state.location,
    navigationType: state.action,
    navigator: history,
    future
  });
}
if (false) {
  HistoryRouter.displayName = "unstable_HistoryRouter";
}
const isBrowser = typeof window !== "undefined" && typeof window.document !== "undefined" && typeof window.document.createElement !== "undefined";
const ABSOLUTE_URL_REGEX = /^(?:[a-z][a-z0-9+.-]*:|\/\/)/i;
const Link = /* @__PURE__ */ reactExports.forwardRef(function LinkWithRef(_ref7, ref) {
  let {
    onClick,
    relative,
    reloadDocument,
    replace: replace2,
    state,
    target,
    to,
    preventScrollReset,
    viewTransition
  } = _ref7, rest = _objectWithoutPropertiesLoose(_ref7, _excluded);
  let {
    basename
  } = reactExports.useContext(NavigationContext);
  let absoluteHref;
  let isExternal = false;
  if (typeof to === "string" && ABSOLUTE_URL_REGEX.test(to)) {
    absoluteHref = to;
    if (isBrowser) {
      try {
        let currentUrl = new URL(window.location.href);
        let targetUrl = to.startsWith("//") ? new URL(currentUrl.protocol + to) : new URL(to);
        let path = stripBasename(targetUrl.pathname, basename);
        if (targetUrl.origin === currentUrl.origin && path != null) {
          to = path + targetUrl.search + targetUrl.hash;
        } else {
          isExternal = true;
        }
      } catch (e) {
        false ? warning(false, '<Link to="' + to + '"> contains an invalid URL which will probably break when clicked - please update to a valid URL path.') : void 0;
      }
    }
  }
  let href = useHref(to, {
    relative
  });
  let internalOnClick = useLinkClickHandler(to, {
    replace: replace2,
    state,
    target,
    preventScrollReset,
    relative,
    viewTransition
  });
  function handleClick(event) {
    if (onClick) onClick(event);
    if (!event.defaultPrevented) {
      internalOnClick(event);
    }
  }
  return (
    // eslint-disable-next-line jsx-a11y/anchor-has-content
    /* @__PURE__ */ reactExports.createElement("a", _extends({}, rest, {
      href: absoluteHref || href,
      onClick: isExternal || reloadDocument ? onClick : handleClick,
      ref,
      target
    }))
  );
});
if (false) {
  Link.displayName = "Link";
}
const NavLink = /* @__PURE__ */ reactExports.forwardRef(function NavLinkWithRef(_ref8, ref) {
  let {
    "aria-current": ariaCurrentProp = "page",
    caseSensitive = false,
    className: classNameProp = "",
    end = false,
    style: styleProp,
    to,
    viewTransition,
    children
  } = _ref8, rest = _objectWithoutPropertiesLoose(_ref8, _excluded2);
  let path = useResolvedPath(to, {
    relative: rest.relative
  });
  let location = useLocation();
  let routerState = reactExports.useContext(DataRouterStateContext);
  let {
    navigator,
    basename
  } = reactExports.useContext(NavigationContext);
  let isTransitioning = routerState != null && // Conditional usage is OK here because the usage of a data router is static
  // eslint-disable-next-line react-hooks/rules-of-hooks
  useViewTransitionState(path) && viewTransition === true;
  let toPathname = navigator.encodeLocation ? navigator.encodeLocation(path).pathname : path.pathname;
  let locationPathname = location.pathname;
  let nextLocationPathname = routerState && routerState.navigation && routerState.navigation.location ? routerState.navigation.location.pathname : null;
  if (!caseSensitive) {
    locationPathname = locationPathname.toLowerCase();
    nextLocationPathname = nextLocationPathname ? nextLocationPathname.toLowerCase() : null;
    toPathname = toPathname.toLowerCase();
  }
  if (nextLocationPathname && basename) {
    nextLocationPathname = stripBasename(nextLocationPathname, basename) || nextLocationPathname;
  }
  const endSlashPosition = toPathname !== "/" && toPathname.endsWith("/") ? toPathname.length - 1 : toPathname.length;
  let isActive = locationPathname === toPathname || !end && locationPathname.startsWith(toPathname) && locationPathname.charAt(endSlashPosition) === "/";
  let isPending = nextLocationPathname != null && (nextLocationPathname === toPathname || !end && nextLocationPathname.startsWith(toPathname) && nextLocationPathname.charAt(toPathname.length) === "/");
  let renderProps = {
    isActive,
    isPending,
    isTransitioning
  };
  let ariaCurrent = isActive ? ariaCurrentProp : void 0;
  let className;
  if (typeof classNameProp === "function") {
    className = classNameProp(renderProps);
  } else {
    className = [classNameProp, isActive ? "active" : null, isPending ? "pending" : null, isTransitioning ? "transitioning" : null].filter(Boolean).join(" ");
  }
  let style = typeof styleProp === "function" ? styleProp(renderProps) : styleProp;
  return /* @__PURE__ */ reactExports.createElement(Link, _extends({}, rest, {
    "aria-current": ariaCurrent,
    className,
    ref,
    style,
    to,
    viewTransition
  }), typeof children === "function" ? children(renderProps) : children);
});
if (false) {
  NavLink.displayName = "NavLink";
}
const Form = /* @__PURE__ */ reactExports.forwardRef((_ref9, forwardedRef) => {
  let {
    fetcherKey,
    navigate,
    reloadDocument,
    replace: replace2,
    state,
    method = defaultMethod,
    action,
    onSubmit,
    relative,
    preventScrollReset,
    viewTransition
  } = _ref9, props = _objectWithoutPropertiesLoose(_ref9, _excluded3);
  let submit = useSubmit();
  let formAction = useFormAction(action, {
    relative
  });
  let formMethod = method.toLowerCase() === "get" ? "get" : "post";
  let submitHandler = (event) => {
    onSubmit && onSubmit(event);
    if (event.defaultPrevented) return;
    event.preventDefault();
    let submitter = event.nativeEvent.submitter;
    let submitMethod = (submitter == null ? void 0 : submitter.getAttribute("formmethod")) || method;
    submit(submitter || event.currentTarget, {
      fetcherKey,
      method: submitMethod,
      navigate,
      replace: replace2,
      state,
      relative,
      preventScrollReset,
      viewTransition
    });
  };
  return /* @__PURE__ */ reactExports.createElement("form", _extends({
    ref: forwardedRef,
    method: formMethod,
    action: formAction,
    onSubmit: reloadDocument ? onSubmit : submitHandler
  }, props));
});
if (false) {
  Form.displayName = "Form";
}
function ScrollRestoration(_ref10) {
  let {
    getKey,
    storageKey
  } = _ref10;
  useScrollRestoration({
    getKey,
    storageKey
  });
  return null;
}
if (false) {
  ScrollRestoration.displayName = "ScrollRestoration";
}
var DataRouterHook;
(function(DataRouterHook2) {
  DataRouterHook2["UseScrollRestoration"] = "useScrollRestoration";
  DataRouterHook2["UseSubmit"] = "useSubmit";
  DataRouterHook2["UseSubmitFetcher"] = "useSubmitFetcher";
  DataRouterHook2["UseFetcher"] = "useFetcher";
  DataRouterHook2["useViewTransitionState"] = "useViewTransitionState";
})(DataRouterHook || (DataRouterHook = {}));
var DataRouterStateHook;
(function(DataRouterStateHook2) {
  DataRouterStateHook2["UseFetcher"] = "useFetcher";
  DataRouterStateHook2["UseFetchers"] = "useFetchers";
  DataRouterStateHook2["UseScrollRestoration"] = "useScrollRestoration";
})(DataRouterStateHook || (DataRouterStateHook = {}));
function getDataRouterConsoleError(hookName) {
  return hookName + " must be used within a data router.  See https://reactrouter.com/v6/routers/picking-a-router.";
}
function useDataRouterContext(hookName) {
  let ctx = reactExports.useContext(DataRouterContext);
  !ctx ? false ? invariant(false, getDataRouterConsoleError(hookName)) : invariant(false) : void 0;
  return ctx;
}
function useDataRouterState(hookName) {
  let state = reactExports.useContext(DataRouterStateContext);
  !state ? false ? invariant(false, getDataRouterConsoleError(hookName)) : invariant(false) : void 0;
  return state;
}
function useLinkClickHandler(to, _temp) {
  let {
    target,
    replace: replaceProp,
    state,
    preventScrollReset,
    relative,
    viewTransition
  } = _temp === void 0 ? {} : _temp;
  let navigate = useNavigate();
  let location = useLocation();
  let path = useResolvedPath(to, {
    relative
  });
  return reactExports.useCallback((event) => {
    if (shouldProcessLinkClick(event, target)) {
      event.preventDefault();
      let replace2 = replaceProp !== void 0 ? replaceProp : createPath(location) === createPath(path);
      navigate(to, {
        replace: replace2,
        state,
        preventScrollReset,
        relative,
        viewTransition
      });
    }
  }, [location, navigate, path, replaceProp, state, target, to, preventScrollReset, relative, viewTransition]);
}
function useSearchParams(defaultInit) {
  false ? warning(typeof URLSearchParams !== "undefined", "You cannot use the `useSearchParams` hook in a browser that does not support the URLSearchParams API. If you need to support Internet Explorer 11, we recommend you load a polyfill such as https://github.com/ungap/url-search-params.") : void 0;
  let defaultSearchParamsRef = reactExports.useRef(createSearchParams(defaultInit));
  let hasSetSearchParamsRef = reactExports.useRef(false);
  let location = useLocation();
  let searchParams = reactExports.useMemo(() => (
    // Only merge in the defaults if we haven't yet called setSearchParams.
    // Once we call that we want those to take precedence, otherwise you can't
    // remove a param with setSearchParams({}) if it has an initial value
    getSearchParamsForLocation(location.search, hasSetSearchParamsRef.current ? null : defaultSearchParamsRef.current)
  ), [location.search]);
  let navigate = useNavigate();
  let setSearchParams = reactExports.useCallback((nextInit, navigateOptions) => {
    const newSearchParams = createSearchParams(typeof nextInit === "function" ? nextInit(searchParams) : nextInit);
    hasSetSearchParamsRef.current = true;
    navigate("?" + newSearchParams, navigateOptions);
  }, [navigate, searchParams]);
  return [searchParams, setSearchParams];
}
function validateClientSideSubmission() {
  if (typeof document === "undefined") {
    throw new Error("You are calling submit during the server render. Try calling submit within a `useEffect` or callback instead.");
  }
}
let fetcherId = 0;
let getUniqueFetcherId = () => "__" + String(++fetcherId) + "__";
function useSubmit() {
  let {
    router
  } = useDataRouterContext(DataRouterHook.UseSubmit);
  let {
    basename
  } = reactExports.useContext(NavigationContext);
  let currentRouteId = useRouteId();
  return reactExports.useCallback(function(target, options) {
    if (options === void 0) {
      options = {};
    }
    validateClientSideSubmission();
    let {
      action,
      method,
      encType,
      formData,
      body
    } = getFormSubmissionInfo(target, basename);
    if (options.navigate === false) {
      let key = options.fetcherKey || getUniqueFetcherId();
      router.fetch(key, currentRouteId, options.action || action, {
        preventScrollReset: options.preventScrollReset,
        formData,
        body,
        formMethod: options.method || method,
        formEncType: options.encType || encType,
        flushSync: options.flushSync
      });
    } else {
      router.navigate(options.action || action, {
        preventScrollReset: options.preventScrollReset,
        formData,
        body,
        formMethod: options.method || method,
        formEncType: options.encType || encType,
        replace: options.replace,
        state: options.state,
        fromRouteId: currentRouteId,
        flushSync: options.flushSync,
        viewTransition: options.viewTransition
      });
    }
  }, [router, basename, currentRouteId]);
}
function useFormAction(action, _temp2) {
  let {
    relative
  } = _temp2 === void 0 ? {} : _temp2;
  let {
    basename
  } = reactExports.useContext(NavigationContext);
  let routeContext = reactExports.useContext(RouteContext);
  !routeContext ? false ? invariant(false, "useFormAction must be used inside a RouteContext") : invariant(false) : void 0;
  let [match] = routeContext.matches.slice(-1);
  let path = _extends({}, useResolvedPath(action ? action : ".", {
    relative
  }));
  let location = useLocation();
  if (action == null) {
    path.search = location.search;
    let params = new URLSearchParams(path.search);
    let indexValues = params.getAll("index");
    let hasNakedIndexParam = indexValues.some((v) => v === "");
    if (hasNakedIndexParam) {
      params.delete("index");
      indexValues.filter((v) => v).forEach((v) => params.append("index", v));
      let qs = params.toString();
      path.search = qs ? "?" + qs : "";
    }
  }
  if ((!action || action === ".") && match.route.index) {
    path.search = path.search ? path.search.replace(/^\?/, "?index&") : "?index";
  }
  if (basename !== "/") {
    path.pathname = path.pathname === "/" ? basename : joinPaths([basename, path.pathname]);
  }
  return createPath(path);
}
function useFetcher(_temp3) {
  var _route$matches;
  let {
    key
  } = _temp3 === void 0 ? {} : _temp3;
  let {
    router
  } = useDataRouterContext(DataRouterHook.UseFetcher);
  let state = useDataRouterState(DataRouterStateHook.UseFetcher);
  let fetcherData = reactExports.useContext(FetchersContext);
  let route = reactExports.useContext(RouteContext);
  let routeId = (_route$matches = route.matches[route.matches.length - 1]) == null ? void 0 : _route$matches.route.id;
  !fetcherData ? false ? invariant(false, "useFetcher must be used inside a FetchersContext") : invariant(false) : void 0;
  !route ? false ? invariant(false, "useFetcher must be used inside a RouteContext") : invariant(false) : void 0;
  !(routeId != null) ? false ? invariant(false, 'useFetcher can only be used on routes that contain a unique "id"') : invariant(false) : void 0;
  let defaultKey = useIdImpl ? useIdImpl() : "";
  let [fetcherKey, setFetcherKey] = reactExports.useState(key || defaultKey);
  if (key && key !== fetcherKey) {
    setFetcherKey(key);
  } else if (!fetcherKey) {
    setFetcherKey(getUniqueFetcherId());
  }
  reactExports.useEffect(() => {
    router.getFetcher(fetcherKey);
    return () => {
      router.deleteFetcher(fetcherKey);
    };
  }, [router, fetcherKey]);
  let load = reactExports.useCallback((href, opts) => {
    !routeId ? false ? invariant(false, "No routeId available for fetcher.load()") : invariant(false) : void 0;
    router.fetch(fetcherKey, routeId, href, opts);
  }, [fetcherKey, routeId, router]);
  let submitImpl = useSubmit();
  let submit = reactExports.useCallback((target, opts) => {
    submitImpl(target, _extends({}, opts, {
      navigate: false,
      fetcherKey
    }));
  }, [fetcherKey, submitImpl]);
  let FetcherForm = reactExports.useMemo(() => {
    let FetcherForm2 = /* @__PURE__ */ reactExports.forwardRef((props, ref) => {
      return /* @__PURE__ */ reactExports.createElement(Form, _extends({}, props, {
        navigate: false,
        fetcherKey,
        ref
      }));
    });
    if (false) {
      FetcherForm2.displayName = "fetcher.Form";
    }
    return FetcherForm2;
  }, [fetcherKey]);
  let fetcher = state.fetchers.get(fetcherKey) || IDLE_FETCHER;
  let data = fetcherData.get(fetcherKey);
  let fetcherWithComponents = reactExports.useMemo(() => _extends({
    Form: FetcherForm,
    submit,
    load
  }, fetcher, {
    data
  }), [FetcherForm, submit, load, fetcher, data]);
  return fetcherWithComponents;
}
function useFetchers() {
  let state = useDataRouterState(DataRouterStateHook.UseFetchers);
  return Array.from(state.fetchers.entries()).map((_ref11) => {
    let [key, fetcher] = _ref11;
    return _extends({}, fetcher, {
      key
    });
  });
}
const SCROLL_RESTORATION_STORAGE_KEY = "react-router-scroll-positions";
let savedScrollPositions = {};
function useScrollRestoration(_temp4) {
  let {
    getKey,
    storageKey
  } = _temp4 === void 0 ? {} : _temp4;
  let {
    router
  } = useDataRouterContext(DataRouterHook.UseScrollRestoration);
  let {
    restoreScrollPosition,
    preventScrollReset
  } = useDataRouterState(DataRouterStateHook.UseScrollRestoration);
  let {
    basename
  } = reactExports.useContext(NavigationContext);
  let location = useLocation();
  let matches = useMatches();
  let navigation = useNavigation();
  reactExports.useEffect(() => {
    window.history.scrollRestoration = "manual";
    return () => {
      window.history.scrollRestoration = "auto";
    };
  }, []);
  usePageHide(reactExports.useCallback(() => {
    if (navigation.state === "idle") {
      let key = (getKey ? getKey(location, matches) : null) || location.key;
      savedScrollPositions[key] = window.scrollY;
    }
    try {
      sessionStorage.setItem(storageKey || SCROLL_RESTORATION_STORAGE_KEY, JSON.stringify(savedScrollPositions));
    } catch (error) {
      false ? warning(false, "Failed to save scroll positions in sessionStorage, <ScrollRestoration /> will not work properly (" + error + ").") : void 0;
    }
    window.history.scrollRestoration = "auto";
  }, [storageKey, getKey, navigation.state, location, matches]));
  if (typeof document !== "undefined") {
    reactExports.useLayoutEffect(() => {
      try {
        let sessionPositions = sessionStorage.getItem(storageKey || SCROLL_RESTORATION_STORAGE_KEY);
        if (sessionPositions) {
          savedScrollPositions = JSON.parse(sessionPositions);
        }
      } catch (e) {
      }
    }, [storageKey]);
    reactExports.useLayoutEffect(() => {
      let getKeyWithoutBasename = getKey && basename !== "/" ? (location2, matches2) => getKey(
        // Strip the basename to match useLocation()
        _extends({}, location2, {
          pathname: stripBasename(location2.pathname, basename) || location2.pathname
        }),
        matches2
      ) : getKey;
      let disableScrollRestoration = router == null ? void 0 : router.enableScrollRestoration(savedScrollPositions, () => window.scrollY, getKeyWithoutBasename);
      return () => disableScrollRestoration && disableScrollRestoration();
    }, [router, basename, getKey]);
    reactExports.useLayoutEffect(() => {
      if (restoreScrollPosition === false) {
        return;
      }
      if (typeof restoreScrollPosition === "number") {
        window.scrollTo(0, restoreScrollPosition);
        return;
      }
      if (location.hash) {
        let el = document.getElementById(decodeURIComponent(location.hash.slice(1)));
        if (el) {
          el.scrollIntoView();
          return;
        }
      }
      if (preventScrollReset === true) {
        return;
      }
      window.scrollTo(0, 0);
    }, [location, restoreScrollPosition, preventScrollReset]);
  }
}
function useBeforeUnload(callback, options) {
  let {
    capture
  } = options || {};
  reactExports.useEffect(() => {
    let opts = capture != null ? {
      capture
    } : void 0;
    window.addEventListener("beforeunload", callback, opts);
    return () => {
      window.removeEventListener("beforeunload", callback, opts);
    };
  }, [callback, capture]);
}
function usePageHide(callback, options) {
  let {
    capture
  } = options || {};
  reactExports.useEffect(() => {
    let opts = capture != null ? {
      capture
    } : void 0;
    window.addEventListener("pagehide", callback, opts);
    return () => {
      window.removeEventListener("pagehide", callback, opts);
    };
  }, [callback, capture]);
}
function usePrompt(_ref12) {
  let {
    when,
    message
  } = _ref12;
  let blocker = useBlocker(when);
  reactExports.useEffect(() => {
    if (blocker.state === "blocked") {
      let proceed = window.confirm(message);
      if (proceed) {
        setTimeout(blocker.proceed, 0);
      } else {
        blocker.reset();
      }
    }
  }, [blocker, message]);
  reactExports.useEffect(() => {
    if (blocker.state === "blocked" && !when) {
      blocker.reset();
    }
  }, [blocker, when]);
}
function useViewTransitionState(to, opts) {
  if (opts === void 0) {
    opts = {};
  }
  let vtContext = reactExports.useContext(ViewTransitionContext);
  !(vtContext != null) ? false ? invariant(false, "`useViewTransitionState` must be used within `react-router-dom`'s `RouterProvider`.  Did you accidentally import `RouterProvider` from `react-router`?") : invariant(false) : void 0;
  let {
    basename
  } = useDataRouterContext(DataRouterHook.useViewTransitionState);
  let path = useResolvedPath(to, {
    relative: opts.relative
  });
  if (!vtContext.isTransitioning) {
    return false;
  }
  let currentPath = stripBasename(vtContext.currentLocation.pathname, basename) || vtContext.currentLocation.pathname;
  let nextPath = stripBasename(vtContext.nextLocation.pathname, basename) || vtContext.nextLocation.pathname;
  return matchPath(path.pathname, nextPath) != null || matchPath(path.pathname, currentPath) != null;
}

const DEFAULT_KINDS = [
  "Pod",
  "Deployment",
  "ReplicaSet",
  "StatefulSet",
  "HelmRelease"
];
const COMMON_KINDS = [
  // Workloads
  "Pod",
  "Deployment",
  "ReplicaSet",
  "StatefulSet",
  "DaemonSet",
  "Job",
  "CronJob",
  // Networking
  "Service",
  "Ingress",
  "NetworkPolicy",
  // Config
  "ConfigMap",
  "Secret",
  // Storage
  "PersistentVolumeClaim",
  // Flux
  "HelmRelease",
  "Kustomization",
  "GitRepository",
  "HelmRepository",
  // ArgoCD
  "Application",
  // RBAC
  "ServiceAccount",
  "Role",
  "RoleBinding",
  "ClusterRole",
  "ClusterRoleBinding",
  // Other
  "Node",
  "Namespace",
  "Event"
];
const DEFAULT_SETTINGS = {
  theme: "dark",
  timeFormat: "24h",
  compactMode: false,
  autoRefresh: "off",
  defaultKinds: DEFAULT_KINDS
};
const STORAGE_KEY = "spectre-settings";
const SettingsContext = reactExports.createContext(void 0);
const loadSettings = () => {
  if (typeof window === "undefined") {
    return DEFAULT_SETTINGS;
  }
  try {
    const stored = window.localStorage.getItem(STORAGE_KEY);
    if (!stored) return DEFAULT_SETTINGS;
    const parsed = JSON.parse(stored);
    return {
      ...DEFAULT_SETTINGS,
      ...parsed
    };
  } catch {
    return DEFAULT_SETTINGS;
  }
};
const SettingsProvider = ({ children }) => {
  const [settings, setSettings] = reactExports.useState(() => loadSettings());
  reactExports.useEffect(() => {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(settings));
  }, [settings]);
  reactExports.useEffect(() => {
    const root = document.documentElement;
    root.setAttribute("data-theme", settings.theme);
  }, [settings.theme]);
  const formatter = reactExports.useMemo(() => {
    return new Intl.DateTimeFormat(void 0, {
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
      hour12: settings.timeFormat === "12h"
    });
  }, [settings.timeFormat]);
  const formatTime = reactExports.useCallback((date) => formatter.format(date), [formatter]);
  const value = {
    ...settings,
    setTheme: (theme) => setSettings((prev) => ({ ...prev, theme })),
    setTimeFormat: (format) => setSettings((prev) => ({ ...prev, timeFormat: format })),
    setCompactMode: (enabled) => setSettings((prev) => ({ ...prev, compactMode: enabled })),
    setAutoRefresh: (value2) => setSettings((prev) => ({ ...prev, autoRefresh: value2 })),
    setDefaultKinds: (kinds) => setSettings((prev) => ({ ...prev, defaultKinds: kinds })),
    formatTime
  };
  return React.createElement(SettingsContext.Provider, { value }, children);
};
const useSettings = () => {
  const context = reactExports.useContext(SettingsContext);
  if (!context) {
    throw new Error("useSettings must be used within SettingsProvider");
  }
  return context;
};

var client$2 = {exports: {}};

var reactDomClient_production = {};

var scheduler$1 = {exports: {}};

var scheduler_production = {};

/**
 * @license React
 * scheduler.production.js
 *
 * Copyright (c) Meta Platforms, Inc. and affiliates.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */

var hasRequiredScheduler_production;

function requireScheduler_production () {
	if (hasRequiredScheduler_production) return scheduler_production;
	hasRequiredScheduler_production = 1;
	(function (exports$1) {
		"use strict";
		function push(heap, node) {
		  var index = heap.length;
		  heap.push(node);
		  a: for (; 0 < index; ) {
		    var parentIndex = (index - 1) >>> 1,
		      parent = heap[parentIndex];
		    if (0 < compare(parent, node))
		      (heap[parentIndex] = node), (heap[index] = parent), (index = parentIndex);
		    else break a;
		  }
		}
		function peek(heap) {
		  return 0 === heap.length ? null : heap[0];
		}
		function pop(heap) {
		  if (0 === heap.length) return null;
		  var first = heap[0],
		    last = heap.pop();
		  if (last !== first) {
		    heap[0] = last;
		    a: for (
		      var index = 0, length = heap.length, halfLength = length >>> 1;
		      index < halfLength;

		    ) {
		      var leftIndex = 2 * (index + 1) - 1,
		        left = heap[leftIndex],
		        rightIndex = leftIndex + 1,
		        right = heap[rightIndex];
		      if (0 > compare(left, last))
		        rightIndex < length && 0 > compare(right, left)
		          ? ((heap[index] = right),
		            (heap[rightIndex] = last),
		            (index = rightIndex))
		          : ((heap[index] = left),
		            (heap[leftIndex] = last),
		            (index = leftIndex));
		      else if (rightIndex < length && 0 > compare(right, last))
		        (heap[index] = right), (heap[rightIndex] = last), (index = rightIndex);
		      else break a;
		    }
		  }
		  return first;
		}
		function compare(a, b) {
		  var diff = a.sortIndex - b.sortIndex;
		  return 0 !== diff ? diff : a.id - b.id;
		}
		exports$1.unstable_now = void 0;
		if ("object" === typeof performance && "function" === typeof performance.now) {
		  var localPerformance = performance;
		  exports$1.unstable_now = function () {
		    return localPerformance.now();
		  };
		} else {
		  var localDate = Date,
		    initialTime = localDate.now();
		  exports$1.unstable_now = function () {
		    return localDate.now() - initialTime;
		  };
		}
		var taskQueue = [],
		  timerQueue = [],
		  taskIdCounter = 1,
		  currentTask = null,
		  currentPriorityLevel = 3,
		  isPerformingWork = !1,
		  isHostCallbackScheduled = !1,
		  isHostTimeoutScheduled = !1,
		  needsPaint = !1,
		  localSetTimeout = "function" === typeof setTimeout ? setTimeout : null,
		  localClearTimeout = "function" === typeof clearTimeout ? clearTimeout : null,
		  localSetImmediate = "undefined" !== typeof setImmediate ? setImmediate : null;
		function advanceTimers(currentTime) {
		  for (var timer = peek(timerQueue); null !== timer; ) {
		    if (null === timer.callback) pop(timerQueue);
		    else if (timer.startTime <= currentTime)
		      pop(timerQueue),
		        (timer.sortIndex = timer.expirationTime),
		        push(taskQueue, timer);
		    else break;
		    timer = peek(timerQueue);
		  }
		}
		function handleTimeout(currentTime) {
		  isHostTimeoutScheduled = !1;
		  advanceTimers(currentTime);
		  if (!isHostCallbackScheduled)
		    if (null !== peek(taskQueue))
		      (isHostCallbackScheduled = !0),
		        isMessageLoopRunning ||
		          ((isMessageLoopRunning = !0), schedulePerformWorkUntilDeadline());
		    else {
		      var firstTimer = peek(timerQueue);
		      null !== firstTimer &&
		        requestHostTimeout(handleTimeout, firstTimer.startTime - currentTime);
		    }
		}
		var isMessageLoopRunning = !1,
		  taskTimeoutID = -1,
		  frameInterval = 5,
		  startTime = -1;
		function shouldYieldToHost() {
		  return needsPaint
		    ? !0
		    : exports$1.unstable_now() - startTime < frameInterval
		      ? !1
		      : !0;
		}
		function performWorkUntilDeadline() {
		  needsPaint = !1;
		  if (isMessageLoopRunning) {
		    var currentTime = exports$1.unstable_now();
		    startTime = currentTime;
		    var hasMoreWork = !0;
		    try {
		      a: {
		        isHostCallbackScheduled = !1;
		        isHostTimeoutScheduled &&
		          ((isHostTimeoutScheduled = !1),
		          localClearTimeout(taskTimeoutID),
		          (taskTimeoutID = -1));
		        isPerformingWork = !0;
		        var previousPriorityLevel = currentPriorityLevel;
		        try {
		          b: {
		            advanceTimers(currentTime);
		            for (
		              currentTask = peek(taskQueue);
		              null !== currentTask &&
		              !(
		                currentTask.expirationTime > currentTime && shouldYieldToHost()
		              );

		            ) {
		              var callback = currentTask.callback;
		              if ("function" === typeof callback) {
		                currentTask.callback = null;
		                currentPriorityLevel = currentTask.priorityLevel;
		                var continuationCallback = callback(
		                  currentTask.expirationTime <= currentTime
		                );
		                currentTime = exports$1.unstable_now();
		                if ("function" === typeof continuationCallback) {
		                  currentTask.callback = continuationCallback;
		                  advanceTimers(currentTime);
		                  hasMoreWork = !0;
		                  break b;
		                }
		                currentTask === peek(taskQueue) && pop(taskQueue);
		                advanceTimers(currentTime);
		              } else pop(taskQueue);
		              currentTask = peek(taskQueue);
		            }
		            if (null !== currentTask) hasMoreWork = !0;
		            else {
		              var firstTimer = peek(timerQueue);
		              null !== firstTimer &&
		                requestHostTimeout(
		                  handleTimeout,
		                  firstTimer.startTime - currentTime
		                );
		              hasMoreWork = !1;
		            }
		          }
		          break a;
		        } finally {
		          (currentTask = null),
		            (currentPriorityLevel = previousPriorityLevel),
		            (isPerformingWork = !1);
		        }
		        hasMoreWork = void 0;
		      }
		    } finally {
		      hasMoreWork
		        ? schedulePerformWorkUntilDeadline()
		        : (isMessageLoopRunning = !1);
		    }
		  }
		}
		var schedulePerformWorkUntilDeadline;
		if ("function" === typeof localSetImmediate)
		  schedulePerformWorkUntilDeadline = function () {
		    localSetImmediate(performWorkUntilDeadline);
		  };
		else if ("undefined" !== typeof MessageChannel) {
		  var channel = new MessageChannel(),
		    port = channel.port2;
		  channel.port1.onmessage = performWorkUntilDeadline;
		  schedulePerformWorkUntilDeadline = function () {
		    port.postMessage(null);
		  };
		} else
		  schedulePerformWorkUntilDeadline = function () {
		    localSetTimeout(performWorkUntilDeadline, 0);
		  };
		function requestHostTimeout(callback, ms) {
		  taskTimeoutID = localSetTimeout(function () {
		    callback(exports$1.unstable_now());
		  }, ms);
		}
		exports$1.unstable_IdlePriority = 5;
		exports$1.unstable_ImmediatePriority = 1;
		exports$1.unstable_LowPriority = 4;
		exports$1.unstable_NormalPriority = 3;
		exports$1.unstable_Profiling = null;
		exports$1.unstable_UserBlockingPriority = 2;
		exports$1.unstable_cancelCallback = function (task) {
		  task.callback = null;
		};
		exports$1.unstable_forceFrameRate = function (fps) {
		  0 > fps || 125 < fps
		    ? console.error(
		        "forceFrameRate takes a positive int between 0 and 125, forcing frame rates higher than 125 fps is not supported"
		      )
		    : (frameInterval = 0 < fps ? Math.floor(1e3 / fps) : 5);
		};
		exports$1.unstable_getCurrentPriorityLevel = function () {
		  return currentPriorityLevel;
		};
		exports$1.unstable_next = function (eventHandler) {
		  switch (currentPriorityLevel) {
		    case 1:
		    case 2:
		    case 3:
		      var priorityLevel = 3;
		      break;
		    default:
		      priorityLevel = currentPriorityLevel;
		  }
		  var previousPriorityLevel = currentPriorityLevel;
		  currentPriorityLevel = priorityLevel;
		  try {
		    return eventHandler();
		  } finally {
		    currentPriorityLevel = previousPriorityLevel;
		  }
		};
		exports$1.unstable_requestPaint = function () {
		  needsPaint = !0;
		};
		exports$1.unstable_runWithPriority = function (priorityLevel, eventHandler) {
		  switch (priorityLevel) {
		    case 1:
		    case 2:
		    case 3:
		    case 4:
		    case 5:
		      break;
		    default:
		      priorityLevel = 3;
		  }
		  var previousPriorityLevel = currentPriorityLevel;
		  currentPriorityLevel = priorityLevel;
		  try {
		    return eventHandler();
		  } finally {
		    currentPriorityLevel = previousPriorityLevel;
		  }
		};
		exports$1.unstable_scheduleCallback = function (
		  priorityLevel,
		  callback,
		  options
		) {
		  var currentTime = exports$1.unstable_now();
		  "object" === typeof options && null !== options
		    ? ((options = options.delay),
		      (options =
		        "number" === typeof options && 0 < options
		          ? currentTime + options
		          : currentTime))
		    : (options = currentTime);
		  switch (priorityLevel) {
		    case 1:
		      var timeout = -1;
		      break;
		    case 2:
		      timeout = 250;
		      break;
		    case 5:
		      timeout = 1073741823;
		      break;
		    case 4:
		      timeout = 1e4;
		      break;
		    default:
		      timeout = 5e3;
		  }
		  timeout = options + timeout;
		  priorityLevel = {
		    id: taskIdCounter++,
		    callback: callback,
		    priorityLevel: priorityLevel,
		    startTime: options,
		    expirationTime: timeout,
		    sortIndex: -1
		  };
		  options > currentTime
		    ? ((priorityLevel.sortIndex = options),
		      push(timerQueue, priorityLevel),
		      null === peek(taskQueue) &&
		        priorityLevel === peek(timerQueue) &&
		        (isHostTimeoutScheduled
		          ? (localClearTimeout(taskTimeoutID), (taskTimeoutID = -1))
		          : (isHostTimeoutScheduled = !0),
		        requestHostTimeout(handleTimeout, options - currentTime)))
		    : ((priorityLevel.sortIndex = timeout),
		      push(taskQueue, priorityLevel),
		      isHostCallbackScheduled ||
		        isPerformingWork ||
		        ((isHostCallbackScheduled = !0),
		        isMessageLoopRunning ||
		          ((isMessageLoopRunning = !0), schedulePerformWorkUntilDeadline())));
		  return priorityLevel;
		};
		exports$1.unstable_shouldYield = shouldYieldToHost;
		exports$1.unstable_wrapCallback = function (callback) {
		  var parentPriorityLevel = currentPriorityLevel;
		  return function () {
		    var previousPriorityLevel = currentPriorityLevel;
		    currentPriorityLevel = parentPriorityLevel;
		    try {
		      return callback.apply(this, arguments);
		    } finally {
		      currentPriorityLevel = previousPriorityLevel;
		    }
		  };
		}; 
	} (scheduler_production));
	return scheduler_production;
}

var scheduler = scheduler$1.exports;

var hasRequiredScheduler;

function requireScheduler () {
	if (hasRequiredScheduler) return scheduler$1.exports;
	hasRequiredScheduler = 1;
	"use strict";
	if (true) {
	  scheduler$1.exports = requireScheduler_production();
	} else {
	  module.exports = require("./cjs/scheduler.development.js");
	}
	return scheduler$1.exports;
}

/**
 * @license React
 * react-dom-client.production.js
 *
 * Copyright (c) Meta Platforms, Inc. and affiliates.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */

var hasRequiredReactDomClient_production;

function requireReactDomClient_production () {
	if (hasRequiredReactDomClient_production) return reactDomClient_production;
	hasRequiredReactDomClient_production = 1;
	/*
	 Modernizr 3.0.0pre (Custom Build) | MIT
	*/
	"use strict";
	var Scheduler = requireScheduler(),
	  React = requireReact(),
	  ReactDOM = requireReactDom();
	function formatProdErrorMessage(code) {
	  var url = "https://react.dev/errors/" + code;
	  if (1 < arguments.length) {
	    url += "?args[]=" + encodeURIComponent(arguments[1]);
	    for (var i = 2; i < arguments.length; i++)
	      url += "&args[]=" + encodeURIComponent(arguments[i]);
	  }
	  return (
	    "Minified React error #" +
	    code +
	    "; visit " +
	    url +
	    " for the full message or use the non-minified dev environment for full errors and additional helpful warnings."
	  );
	}
	function isValidContainer(node) {
	  return !(
	    !node ||
	    (1 !== node.nodeType && 9 !== node.nodeType && 11 !== node.nodeType)
	  );
	}
	function getNearestMountedFiber(fiber) {
	  var node = fiber,
	    nearestMounted = fiber;
	  if (fiber.alternate) for (; node.return; ) node = node.return;
	  else {
	    fiber = node;
	    do
	      (node = fiber),
	        0 !== (node.flags & 4098) && (nearestMounted = node.return),
	        (fiber = node.return);
	    while (fiber);
	  }
	  return 3 === node.tag ? nearestMounted : null;
	}
	function getSuspenseInstanceFromFiber(fiber) {
	  if (13 === fiber.tag) {
	    var suspenseState = fiber.memoizedState;
	    null === suspenseState &&
	      ((fiber = fiber.alternate),
	      null !== fiber && (suspenseState = fiber.memoizedState));
	    if (null !== suspenseState) return suspenseState.dehydrated;
	  }
	  return null;
	}
	function getActivityInstanceFromFiber(fiber) {
	  if (31 === fiber.tag) {
	    var activityState = fiber.memoizedState;
	    null === activityState &&
	      ((fiber = fiber.alternate),
	      null !== fiber && (activityState = fiber.memoizedState));
	    if (null !== activityState) return activityState.dehydrated;
	  }
	  return null;
	}
	function assertIsMounted(fiber) {
	  if (getNearestMountedFiber(fiber) !== fiber)
	    throw Error(formatProdErrorMessage(188));
	}
	function findCurrentFiberUsingSlowPath(fiber) {
	  var alternate = fiber.alternate;
	  if (!alternate) {
	    alternate = getNearestMountedFiber(fiber);
	    if (null === alternate) throw Error(formatProdErrorMessage(188));
	    return alternate !== fiber ? null : fiber;
	  }
	  for (var a = fiber, b = alternate; ; ) {
	    var parentA = a.return;
	    if (null === parentA) break;
	    var parentB = parentA.alternate;
	    if (null === parentB) {
	      b = parentA.return;
	      if (null !== b) {
	        a = b;
	        continue;
	      }
	      break;
	    }
	    if (parentA.child === parentB.child) {
	      for (parentB = parentA.child; parentB; ) {
	        if (parentB === a) return assertIsMounted(parentA), fiber;
	        if (parentB === b) return assertIsMounted(parentA), alternate;
	        parentB = parentB.sibling;
	      }
	      throw Error(formatProdErrorMessage(188));
	    }
	    if (a.return !== b.return) (a = parentA), (b = parentB);
	    else {
	      for (var didFindChild = !1, child$0 = parentA.child; child$0; ) {
	        if (child$0 === a) {
	          didFindChild = !0;
	          a = parentA;
	          b = parentB;
	          break;
	        }
	        if (child$0 === b) {
	          didFindChild = !0;
	          b = parentA;
	          a = parentB;
	          break;
	        }
	        child$0 = child$0.sibling;
	      }
	      if (!didFindChild) {
	        for (child$0 = parentB.child; child$0; ) {
	          if (child$0 === a) {
	            didFindChild = !0;
	            a = parentB;
	            b = parentA;
	            break;
	          }
	          if (child$0 === b) {
	            didFindChild = !0;
	            b = parentB;
	            a = parentA;
	            break;
	          }
	          child$0 = child$0.sibling;
	        }
	        if (!didFindChild) throw Error(formatProdErrorMessage(189));
	      }
	    }
	    if (a.alternate !== b) throw Error(formatProdErrorMessage(190));
	  }
	  if (3 !== a.tag) throw Error(formatProdErrorMessage(188));
	  return a.stateNode.current === a ? fiber : alternate;
	}
	function findCurrentHostFiberImpl(node) {
	  var tag = node.tag;
	  if (5 === tag || 26 === tag || 27 === tag || 6 === tag) return node;
	  for (node = node.child; null !== node; ) {
	    tag = findCurrentHostFiberImpl(node);
	    if (null !== tag) return tag;
	    node = node.sibling;
	  }
	  return null;
	}
	var assign = Object.assign,
	  REACT_LEGACY_ELEMENT_TYPE = Symbol.for("react.element"),
	  REACT_ELEMENT_TYPE = Symbol.for("react.transitional.element"),
	  REACT_PORTAL_TYPE = Symbol.for("react.portal"),
	  REACT_FRAGMENT_TYPE = Symbol.for("react.fragment"),
	  REACT_STRICT_MODE_TYPE = Symbol.for("react.strict_mode"),
	  REACT_PROFILER_TYPE = Symbol.for("react.profiler"),
	  REACT_CONSUMER_TYPE = Symbol.for("react.consumer"),
	  REACT_CONTEXT_TYPE = Symbol.for("react.context"),
	  REACT_FORWARD_REF_TYPE = Symbol.for("react.forward_ref"),
	  REACT_SUSPENSE_TYPE = Symbol.for("react.suspense"),
	  REACT_SUSPENSE_LIST_TYPE = Symbol.for("react.suspense_list"),
	  REACT_MEMO_TYPE = Symbol.for("react.memo"),
	  REACT_LAZY_TYPE = Symbol.for("react.lazy");
	Symbol.for("react.scope");
	var REACT_ACTIVITY_TYPE = Symbol.for("react.activity");
	Symbol.for("react.legacy_hidden");
	Symbol.for("react.tracing_marker");
	var REACT_MEMO_CACHE_SENTINEL = Symbol.for("react.memo_cache_sentinel");
	Symbol.for("react.view_transition");
	var MAYBE_ITERATOR_SYMBOL = Symbol.iterator;
	function getIteratorFn(maybeIterable) {
	  if (null === maybeIterable || "object" !== typeof maybeIterable) return null;
	  maybeIterable =
	    (MAYBE_ITERATOR_SYMBOL && maybeIterable[MAYBE_ITERATOR_SYMBOL]) ||
	    maybeIterable["@@iterator"];
	  return "function" === typeof maybeIterable ? maybeIterable : null;
	}
	var REACT_CLIENT_REFERENCE = Symbol.for("react.client.reference");
	function getComponentNameFromType(type) {
	  if (null == type) return null;
	  if ("function" === typeof type)
	    return type.$$typeof === REACT_CLIENT_REFERENCE
	      ? null
	      : type.displayName || type.name || null;
	  if ("string" === typeof type) return type;
	  switch (type) {
	    case REACT_FRAGMENT_TYPE:
	      return "Fragment";
	    case REACT_PROFILER_TYPE:
	      return "Profiler";
	    case REACT_STRICT_MODE_TYPE:
	      return "StrictMode";
	    case REACT_SUSPENSE_TYPE:
	      return "Suspense";
	    case REACT_SUSPENSE_LIST_TYPE:
	      return "SuspenseList";
	    case REACT_ACTIVITY_TYPE:
	      return "Activity";
	  }
	  if ("object" === typeof type)
	    switch (type.$$typeof) {
	      case REACT_PORTAL_TYPE:
	        return "Portal";
	      case REACT_CONTEXT_TYPE:
	        return type.displayName || "Context";
	      case REACT_CONSUMER_TYPE:
	        return (type._context.displayName || "Context") + ".Consumer";
	      case REACT_FORWARD_REF_TYPE:
	        var innerType = type.render;
	        type = type.displayName;
	        type ||
	          ((type = innerType.displayName || innerType.name || ""),
	          (type = "" !== type ? "ForwardRef(" + type + ")" : "ForwardRef"));
	        return type;
	      case REACT_MEMO_TYPE:
	        return (
	          (innerType = type.displayName || null),
	          null !== innerType
	            ? innerType
	            : getComponentNameFromType(type.type) || "Memo"
	        );
	      case REACT_LAZY_TYPE:
	        innerType = type._payload;
	        type = type._init;
	        try {
	          return getComponentNameFromType(type(innerType));
	        } catch (x) {}
	    }
	  return null;
	}
	var isArrayImpl = Array.isArray,
	  ReactSharedInternals =
	    React.__CLIENT_INTERNALS_DO_NOT_USE_OR_WARN_USERS_THEY_CANNOT_UPGRADE,
	  ReactDOMSharedInternals =
	    ReactDOM.__DOM_INTERNALS_DO_NOT_USE_OR_WARN_USERS_THEY_CANNOT_UPGRADE,
	  sharedNotPendingObject = {
	    pending: !1,
	    data: null,
	    method: null,
	    action: null
	  },
	  valueStack = [],
	  index = -1;
	function createCursor(defaultValue) {
	  return { current: defaultValue };
	}
	function pop(cursor) {
	  0 > index ||
	    ((cursor.current = valueStack[index]), (valueStack[index] = null), index--);
	}
	function push(cursor, value) {
	  index++;
	  valueStack[index] = cursor.current;
	  cursor.current = value;
	}
	var contextStackCursor = createCursor(null),
	  contextFiberStackCursor = createCursor(null),
	  rootInstanceStackCursor = createCursor(null),
	  hostTransitionProviderCursor = createCursor(null);
	function pushHostContainer(fiber, nextRootInstance) {
	  push(rootInstanceStackCursor, nextRootInstance);
	  push(contextFiberStackCursor, fiber);
	  push(contextStackCursor, null);
	  switch (nextRootInstance.nodeType) {
	    case 9:
	    case 11:
	      fiber = (fiber = nextRootInstance.documentElement)
	        ? (fiber = fiber.namespaceURI)
	          ? getOwnHostContext(fiber)
	          : 0
	        : 0;
	      break;
	    default:
	      if (
	        ((fiber = nextRootInstance.tagName),
	        (nextRootInstance = nextRootInstance.namespaceURI))
	      )
	        (nextRootInstance = getOwnHostContext(nextRootInstance)),
	          (fiber = getChildHostContextProd(nextRootInstance, fiber));
	      else
	        switch (fiber) {
	          case "svg":
	            fiber = 1;
	            break;
	          case "math":
	            fiber = 2;
	            break;
	          default:
	            fiber = 0;
	        }
	  }
	  pop(contextStackCursor);
	  push(contextStackCursor, fiber);
	}
	function popHostContainer() {
	  pop(contextStackCursor);
	  pop(contextFiberStackCursor);
	  pop(rootInstanceStackCursor);
	}
	function pushHostContext(fiber) {
	  null !== fiber.memoizedState && push(hostTransitionProviderCursor, fiber);
	  var context = contextStackCursor.current;
	  var JSCompiler_inline_result = getChildHostContextProd(context, fiber.type);
	  context !== JSCompiler_inline_result &&
	    (push(contextFiberStackCursor, fiber),
	    push(contextStackCursor, JSCompiler_inline_result));
	}
	function popHostContext(fiber) {
	  contextFiberStackCursor.current === fiber &&
	    (pop(contextStackCursor), pop(contextFiberStackCursor));
	  hostTransitionProviderCursor.current === fiber &&
	    (pop(hostTransitionProviderCursor),
	    (HostTransitionContext._currentValue = sharedNotPendingObject));
	}
	var prefix, suffix;
	function describeBuiltInComponentFrame(name) {
	  if (void 0 === prefix)
	    try {
	      throw Error();
	    } catch (x) {
	      var match = x.stack.trim().match(/\n( *(at )?)/);
	      prefix = (match && match[1]) || "";
	      suffix =
	        -1 < x.stack.indexOf("\n    at")
	          ? " (<anonymous>)"
	          : -1 < x.stack.indexOf("@")
	            ? "@unknown:0:0"
	            : "";
	    }
	  return "\n" + prefix + name + suffix;
	}
	var reentry = !1;
	function describeNativeComponentFrame(fn, construct) {
	  if (!fn || reentry) return "";
	  reentry = !0;
	  var previousPrepareStackTrace = Error.prepareStackTrace;
	  Error.prepareStackTrace = void 0;
	  try {
	    var RunInRootFrame = {
	      DetermineComponentFrameRoot: function () {
	        try {
	          if (construct) {
	            var Fake = function () {
	              throw Error();
	            };
	            Object.defineProperty(Fake.prototype, "props", {
	              set: function () {
	                throw Error();
	              }
	            });
	            if ("object" === typeof Reflect && Reflect.construct) {
	              try {
	                Reflect.construct(Fake, []);
	              } catch (x) {
	                var control = x;
	              }
	              Reflect.construct(fn, [], Fake);
	            } else {
	              try {
	                Fake.call();
	              } catch (x$1) {
	                control = x$1;
	              }
	              fn.call(Fake.prototype);
	            }
	          } else {
	            try {
	              throw Error();
	            } catch (x$2) {
	              control = x$2;
	            }
	            (Fake = fn()) &&
	              "function" === typeof Fake.catch &&
	              Fake.catch(function () {});
	          }
	        } catch (sample) {
	          if (sample && control && "string" === typeof sample.stack)
	            return [sample.stack, control.stack];
	        }
	        return [null, null];
	      }
	    };
	    RunInRootFrame.DetermineComponentFrameRoot.displayName =
	      "DetermineComponentFrameRoot";
	    var namePropDescriptor = Object.getOwnPropertyDescriptor(
	      RunInRootFrame.DetermineComponentFrameRoot,
	      "name"
	    );
	    namePropDescriptor &&
	      namePropDescriptor.configurable &&
	      Object.defineProperty(
	        RunInRootFrame.DetermineComponentFrameRoot,
	        "name",
	        { value: "DetermineComponentFrameRoot" }
	      );
	    var _RunInRootFrame$Deter = RunInRootFrame.DetermineComponentFrameRoot(),
	      sampleStack = _RunInRootFrame$Deter[0],
	      controlStack = _RunInRootFrame$Deter[1];
	    if (sampleStack && controlStack) {
	      var sampleLines = sampleStack.split("\n"),
	        controlLines = controlStack.split("\n");
	      for (
	        namePropDescriptor = RunInRootFrame = 0;
	        RunInRootFrame < sampleLines.length &&
	        !sampleLines[RunInRootFrame].includes("DetermineComponentFrameRoot");

	      )
	        RunInRootFrame++;
	      for (
	        ;
	        namePropDescriptor < controlLines.length &&
	        !controlLines[namePropDescriptor].includes(
	          "DetermineComponentFrameRoot"
	        );

	      )
	        namePropDescriptor++;
	      if (
	        RunInRootFrame === sampleLines.length ||
	        namePropDescriptor === controlLines.length
	      )
	        for (
	          RunInRootFrame = sampleLines.length - 1,
	            namePropDescriptor = controlLines.length - 1;
	          1 <= RunInRootFrame &&
	          0 <= namePropDescriptor &&
	          sampleLines[RunInRootFrame] !== controlLines[namePropDescriptor];

	        )
	          namePropDescriptor--;
	      for (
	        ;
	        1 <= RunInRootFrame && 0 <= namePropDescriptor;
	        RunInRootFrame--, namePropDescriptor--
	      )
	        if (sampleLines[RunInRootFrame] !== controlLines[namePropDescriptor]) {
	          if (1 !== RunInRootFrame || 1 !== namePropDescriptor) {
	            do
	              if (
	                (RunInRootFrame--,
	                namePropDescriptor--,
	                0 > namePropDescriptor ||
	                  sampleLines[RunInRootFrame] !==
	                    controlLines[namePropDescriptor])
	              ) {
	                var frame =
	                  "\n" +
	                  sampleLines[RunInRootFrame].replace(" at new ", " at ");
	                fn.displayName &&
	                  frame.includes("<anonymous>") &&
	                  (frame = frame.replace("<anonymous>", fn.displayName));
	                return frame;
	              }
	            while (1 <= RunInRootFrame && 0 <= namePropDescriptor);
	          }
	          break;
	        }
	    }
	  } finally {
	    (reentry = !1), (Error.prepareStackTrace = previousPrepareStackTrace);
	  }
	  return (previousPrepareStackTrace = fn ? fn.displayName || fn.name : "")
	    ? describeBuiltInComponentFrame(previousPrepareStackTrace)
	    : "";
	}
	function describeFiber(fiber, childFiber) {
	  switch (fiber.tag) {
	    case 26:
	    case 27:
	    case 5:
	      return describeBuiltInComponentFrame(fiber.type);
	    case 16:
	      return describeBuiltInComponentFrame("Lazy");
	    case 13:
	      return fiber.child !== childFiber && null !== childFiber
	        ? describeBuiltInComponentFrame("Suspense Fallback")
	        : describeBuiltInComponentFrame("Suspense");
	    case 19:
	      return describeBuiltInComponentFrame("SuspenseList");
	    case 0:
	    case 15:
	      return describeNativeComponentFrame(fiber.type, !1);
	    case 11:
	      return describeNativeComponentFrame(fiber.type.render, !1);
	    case 1:
	      return describeNativeComponentFrame(fiber.type, !0);
	    case 31:
	      return describeBuiltInComponentFrame("Activity");
	    default:
	      return "";
	  }
	}
	function getStackByFiberInDevAndProd(workInProgress) {
	  try {
	    var info = "",
	      previous = null;
	    do
	      (info += describeFiber(workInProgress, previous)),
	        (previous = workInProgress),
	        (workInProgress = workInProgress.return);
	    while (workInProgress);
	    return info;
	  } catch (x) {
	    return "\nError generating stack: " + x.message + "\n" + x.stack;
	  }
	}
	var hasOwnProperty = Object.prototype.hasOwnProperty,
	  scheduleCallback$3 = Scheduler.unstable_scheduleCallback,
	  cancelCallback$1 = Scheduler.unstable_cancelCallback,
	  shouldYield = Scheduler.unstable_shouldYield,
	  requestPaint = Scheduler.unstable_requestPaint,
	  now = Scheduler.unstable_now,
	  getCurrentPriorityLevel = Scheduler.unstable_getCurrentPriorityLevel,
	  ImmediatePriority = Scheduler.unstable_ImmediatePriority,
	  UserBlockingPriority = Scheduler.unstable_UserBlockingPriority,
	  NormalPriority$1 = Scheduler.unstable_NormalPriority,
	  LowPriority = Scheduler.unstable_LowPriority,
	  IdlePriority = Scheduler.unstable_IdlePriority,
	  log$1 = Scheduler.log,
	  unstable_setDisableYieldValue = Scheduler.unstable_setDisableYieldValue,
	  rendererID = null,
	  injectedHook = null;
	function setIsStrictModeForDevtools(newIsStrictMode) {
	  "function" === typeof log$1 && unstable_setDisableYieldValue(newIsStrictMode);
	  if (injectedHook && "function" === typeof injectedHook.setStrictMode)
	    try {
	      injectedHook.setStrictMode(rendererID, newIsStrictMode);
	    } catch (err) {}
	}
	var clz32 = Math.clz32 ? Math.clz32 : clz32Fallback,
	  log = Math.log,
	  LN2 = Math.LN2;
	function clz32Fallback(x) {
	  x >>>= 0;
	  return 0 === x ? 32 : (31 - ((log(x) / LN2) | 0)) | 0;
	}
	var nextTransitionUpdateLane = 256,
	  nextTransitionDeferredLane = 262144,
	  nextRetryLane = 4194304;
	function getHighestPriorityLanes(lanes) {
	  var pendingSyncLanes = lanes & 42;
	  if (0 !== pendingSyncLanes) return pendingSyncLanes;
	  switch (lanes & -lanes) {
	    case 1:
	      return 1;
	    case 2:
	      return 2;
	    case 4:
	      return 4;
	    case 8:
	      return 8;
	    case 16:
	      return 16;
	    case 32:
	      return 32;
	    case 64:
	      return 64;
	    case 128:
	      return 128;
	    case 256:
	    case 512:
	    case 1024:
	    case 2048:
	    case 4096:
	    case 8192:
	    case 16384:
	    case 32768:
	    case 65536:
	    case 131072:
	      return lanes & 261888;
	    case 262144:
	    case 524288:
	    case 1048576:
	    case 2097152:
	      return lanes & 3932160;
	    case 4194304:
	    case 8388608:
	    case 16777216:
	    case 33554432:
	      return lanes & 62914560;
	    case 67108864:
	      return 67108864;
	    case 134217728:
	      return 134217728;
	    case 268435456:
	      return 268435456;
	    case 536870912:
	      return 536870912;
	    case 1073741824:
	      return 0;
	    default:
	      return lanes;
	  }
	}
	function getNextLanes(root, wipLanes, rootHasPendingCommit) {
	  var pendingLanes = root.pendingLanes;
	  if (0 === pendingLanes) return 0;
	  var nextLanes = 0,
	    suspendedLanes = root.suspendedLanes,
	    pingedLanes = root.pingedLanes;
	  root = root.warmLanes;
	  var nonIdlePendingLanes = pendingLanes & 134217727;
	  0 !== nonIdlePendingLanes
	    ? ((pendingLanes = nonIdlePendingLanes & ~suspendedLanes),
	      0 !== pendingLanes
	        ? (nextLanes = getHighestPriorityLanes(pendingLanes))
	        : ((pingedLanes &= nonIdlePendingLanes),
	          0 !== pingedLanes
	            ? (nextLanes = getHighestPriorityLanes(pingedLanes))
	            : rootHasPendingCommit ||
	              ((rootHasPendingCommit = nonIdlePendingLanes & ~root),
	              0 !== rootHasPendingCommit &&
	                (nextLanes = getHighestPriorityLanes(rootHasPendingCommit)))))
	    : ((nonIdlePendingLanes = pendingLanes & ~suspendedLanes),
	      0 !== nonIdlePendingLanes
	        ? (nextLanes = getHighestPriorityLanes(nonIdlePendingLanes))
	        : 0 !== pingedLanes
	          ? (nextLanes = getHighestPriorityLanes(pingedLanes))
	          : rootHasPendingCommit ||
	            ((rootHasPendingCommit = pendingLanes & ~root),
	            0 !== rootHasPendingCommit &&
	              (nextLanes = getHighestPriorityLanes(rootHasPendingCommit))));
	  return 0 === nextLanes
	    ? 0
	    : 0 !== wipLanes &&
	        wipLanes !== nextLanes &&
	        0 === (wipLanes & suspendedLanes) &&
	        ((suspendedLanes = nextLanes & -nextLanes),
	        (rootHasPendingCommit = wipLanes & -wipLanes),
	        suspendedLanes >= rootHasPendingCommit ||
	          (32 === suspendedLanes && 0 !== (rootHasPendingCommit & 4194048)))
	      ? wipLanes
	      : nextLanes;
	}
	function checkIfRootIsPrerendering(root, renderLanes) {
	  return (
	    0 ===
	    (root.pendingLanes &
	      ~(root.suspendedLanes & ~root.pingedLanes) &
	      renderLanes)
	  );
	}
	function computeExpirationTime(lane, currentTime) {
	  switch (lane) {
	    case 1:
	    case 2:
	    case 4:
	    case 8:
	    case 64:
	      return currentTime + 250;
	    case 16:
	    case 32:
	    case 128:
	    case 256:
	    case 512:
	    case 1024:
	    case 2048:
	    case 4096:
	    case 8192:
	    case 16384:
	    case 32768:
	    case 65536:
	    case 131072:
	    case 262144:
	    case 524288:
	    case 1048576:
	    case 2097152:
	      return currentTime + 5e3;
	    case 4194304:
	    case 8388608:
	    case 16777216:
	    case 33554432:
	      return -1;
	    case 67108864:
	    case 134217728:
	    case 268435456:
	    case 536870912:
	    case 1073741824:
	      return -1;
	    default:
	      return -1;
	  }
	}
	function claimNextRetryLane() {
	  var lane = nextRetryLane;
	  nextRetryLane <<= 1;
	  0 === (nextRetryLane & 62914560) && (nextRetryLane = 4194304);
	  return lane;
	}
	function createLaneMap(initial) {
	  for (var laneMap = [], i = 0; 31 > i; i++) laneMap.push(initial);
	  return laneMap;
	}
	function markRootUpdated$1(root, updateLane) {
	  root.pendingLanes |= updateLane;
	  268435456 !== updateLane &&
	    ((root.suspendedLanes = 0), (root.pingedLanes = 0), (root.warmLanes = 0));
	}
	function markRootFinished(
	  root,
	  finishedLanes,
	  remainingLanes,
	  spawnedLane,
	  updatedLanes,
	  suspendedRetryLanes
	) {
	  var previouslyPendingLanes = root.pendingLanes;
	  root.pendingLanes = remainingLanes;
	  root.suspendedLanes = 0;
	  root.pingedLanes = 0;
	  root.warmLanes = 0;
	  root.expiredLanes &= remainingLanes;
	  root.entangledLanes &= remainingLanes;
	  root.errorRecoveryDisabledLanes &= remainingLanes;
	  root.shellSuspendCounter = 0;
	  var entanglements = root.entanglements,
	    expirationTimes = root.expirationTimes,
	    hiddenUpdates = root.hiddenUpdates;
	  for (
	    remainingLanes = previouslyPendingLanes & ~remainingLanes;
	    0 < remainingLanes;

	  ) {
	    var index$7 = 31 - clz32(remainingLanes),
	      lane = 1 << index$7;
	    entanglements[index$7] = 0;
	    expirationTimes[index$7] = -1;
	    var hiddenUpdatesForLane = hiddenUpdates[index$7];
	    if (null !== hiddenUpdatesForLane)
	      for (
	        hiddenUpdates[index$7] = null, index$7 = 0;
	        index$7 < hiddenUpdatesForLane.length;
	        index$7++
	      ) {
	        var update = hiddenUpdatesForLane[index$7];
	        null !== update && (update.lane &= -536870913);
	      }
	    remainingLanes &= ~lane;
	  }
	  0 !== spawnedLane && markSpawnedDeferredLane(root, spawnedLane, 0);
	  0 !== suspendedRetryLanes &&
	    0 === updatedLanes &&
	    0 !== root.tag &&
	    (root.suspendedLanes |=
	      suspendedRetryLanes & ~(previouslyPendingLanes & ~finishedLanes));
	}
	function markSpawnedDeferredLane(root, spawnedLane, entangledLanes) {
	  root.pendingLanes |= spawnedLane;
	  root.suspendedLanes &= ~spawnedLane;
	  var spawnedLaneIndex = 31 - clz32(spawnedLane);
	  root.entangledLanes |= spawnedLane;
	  root.entanglements[spawnedLaneIndex] =
	    root.entanglements[spawnedLaneIndex] |
	    1073741824 |
	    (entangledLanes & 261930);
	}
	function markRootEntangled(root, entangledLanes) {
	  var rootEntangledLanes = (root.entangledLanes |= entangledLanes);
	  for (root = root.entanglements; rootEntangledLanes; ) {
	    var index$8 = 31 - clz32(rootEntangledLanes),
	      lane = 1 << index$8;
	    (lane & entangledLanes) | (root[index$8] & entangledLanes) &&
	      (root[index$8] |= entangledLanes);
	    rootEntangledLanes &= ~lane;
	  }
	}
	function getBumpedLaneForHydration(root, renderLanes) {
	  var renderLane = renderLanes & -renderLanes;
	  renderLane =
	    0 !== (renderLane & 42) ? 1 : getBumpedLaneForHydrationByLane(renderLane);
	  return 0 !== (renderLane & (root.suspendedLanes | renderLanes))
	    ? 0
	    : renderLane;
	}
	function getBumpedLaneForHydrationByLane(lane) {
	  switch (lane) {
	    case 2:
	      lane = 1;
	      break;
	    case 8:
	      lane = 4;
	      break;
	    case 32:
	      lane = 16;
	      break;
	    case 256:
	    case 512:
	    case 1024:
	    case 2048:
	    case 4096:
	    case 8192:
	    case 16384:
	    case 32768:
	    case 65536:
	    case 131072:
	    case 262144:
	    case 524288:
	    case 1048576:
	    case 2097152:
	    case 4194304:
	    case 8388608:
	    case 16777216:
	    case 33554432:
	      lane = 128;
	      break;
	    case 268435456:
	      lane = 134217728;
	      break;
	    default:
	      lane = 0;
	  }
	  return lane;
	}
	function lanesToEventPriority(lanes) {
	  lanes &= -lanes;
	  return 2 < lanes
	    ? 8 < lanes
	      ? 0 !== (lanes & 134217727)
	        ? 32
	        : 268435456
	      : 8
	    : 2;
	}
	function resolveUpdatePriority() {
	  var updatePriority = ReactDOMSharedInternals.p;
	  if (0 !== updatePriority) return updatePriority;
	  updatePriority = window.event;
	  return void 0 === updatePriority ? 32 : getEventPriority(updatePriority.type);
	}
	function runWithPriority(priority, fn) {
	  var previousPriority = ReactDOMSharedInternals.p;
	  try {
	    return (ReactDOMSharedInternals.p = priority), fn();
	  } finally {
	    ReactDOMSharedInternals.p = previousPriority;
	  }
	}
	var randomKey = Math.random().toString(36).slice(2),
	  internalInstanceKey = "__reactFiber$" + randomKey,
	  internalPropsKey = "__reactProps$" + randomKey,
	  internalContainerInstanceKey = "__reactContainer$" + randomKey,
	  internalEventHandlersKey = "__reactEvents$" + randomKey,
	  internalEventHandlerListenersKey = "__reactListeners$" + randomKey,
	  internalEventHandlesSetKey = "__reactHandles$" + randomKey,
	  internalRootNodeResourcesKey = "__reactResources$" + randomKey,
	  internalHoistableMarker = "__reactMarker$" + randomKey;
	function detachDeletedInstance(node) {
	  delete node[internalInstanceKey];
	  delete node[internalPropsKey];
	  delete node[internalEventHandlersKey];
	  delete node[internalEventHandlerListenersKey];
	  delete node[internalEventHandlesSetKey];
	}
	function getClosestInstanceFromNode(targetNode) {
	  var targetInst = targetNode[internalInstanceKey];
	  if (targetInst) return targetInst;
	  for (var parentNode = targetNode.parentNode; parentNode; ) {
	    if (
	      (targetInst =
	        parentNode[internalContainerInstanceKey] ||
	        parentNode[internalInstanceKey])
	    ) {
	      parentNode = targetInst.alternate;
	      if (
	        null !== targetInst.child ||
	        (null !== parentNode && null !== parentNode.child)
	      )
	        for (
	          targetNode = getParentHydrationBoundary(targetNode);
	          null !== targetNode;

	        ) {
	          if ((parentNode = targetNode[internalInstanceKey])) return parentNode;
	          targetNode = getParentHydrationBoundary(targetNode);
	        }
	      return targetInst;
	    }
	    targetNode = parentNode;
	    parentNode = targetNode.parentNode;
	  }
	  return null;
	}
	function getInstanceFromNode(node) {
	  if (
	    (node = node[internalInstanceKey] || node[internalContainerInstanceKey])
	  ) {
	    var tag = node.tag;
	    if (
	      5 === tag ||
	      6 === tag ||
	      13 === tag ||
	      31 === tag ||
	      26 === tag ||
	      27 === tag ||
	      3 === tag
	    )
	      return node;
	  }
	  return null;
	}
	function getNodeFromInstance(inst) {
	  var tag = inst.tag;
	  if (5 === tag || 26 === tag || 27 === tag || 6 === tag) return inst.stateNode;
	  throw Error(formatProdErrorMessage(33));
	}
	function getResourcesFromRoot(root) {
	  var resources = root[internalRootNodeResourcesKey];
	  resources ||
	    (resources = root[internalRootNodeResourcesKey] =
	      { hoistableStyles: new Map(), hoistableScripts: new Map() });
	  return resources;
	}
	function markNodeAsHoistable(node) {
	  node[internalHoistableMarker] = !0;
	}
	var allNativeEvents = new Set(),
	  registrationNameDependencies = {};
	function registerTwoPhaseEvent(registrationName, dependencies) {
	  registerDirectEvent(registrationName, dependencies);
	  registerDirectEvent(registrationName + "Capture", dependencies);
	}
	function registerDirectEvent(registrationName, dependencies) {
	  registrationNameDependencies[registrationName] = dependencies;
	  for (
	    registrationName = 0;
	    registrationName < dependencies.length;
	    registrationName++
	  )
	    allNativeEvents.add(dependencies[registrationName]);
	}
	var VALID_ATTRIBUTE_NAME_REGEX = RegExp(
	    "^[:A-Z_a-z\\u00C0-\\u00D6\\u00D8-\\u00F6\\u00F8-\\u02FF\\u0370-\\u037D\\u037F-\\u1FFF\\u200C-\\u200D\\u2070-\\u218F\\u2C00-\\u2FEF\\u3001-\\uD7FF\\uF900-\\uFDCF\\uFDF0-\\uFFFD][:A-Z_a-z\\u00C0-\\u00D6\\u00D8-\\u00F6\\u00F8-\\u02FF\\u0370-\\u037D\\u037F-\\u1FFF\\u200C-\\u200D\\u2070-\\u218F\\u2C00-\\u2FEF\\u3001-\\uD7FF\\uF900-\\uFDCF\\uFDF0-\\uFFFD\\-.0-9\\u00B7\\u0300-\\u036F\\u203F-\\u2040]*$"
	  ),
	  illegalAttributeNameCache = {},
	  validatedAttributeNameCache = {};
	function isAttributeNameSafe(attributeName) {
	  if (hasOwnProperty.call(validatedAttributeNameCache, attributeName))
	    return !0;
	  if (hasOwnProperty.call(illegalAttributeNameCache, attributeName)) return !1;
	  if (VALID_ATTRIBUTE_NAME_REGEX.test(attributeName))
	    return (validatedAttributeNameCache[attributeName] = !0);
	  illegalAttributeNameCache[attributeName] = !0;
	  return !1;
	}
	function setValueForAttribute(node, name, value) {
	  if (isAttributeNameSafe(name))
	    if (null === value) node.removeAttribute(name);
	    else {
	      switch (typeof value) {
	        case "undefined":
	        case "function":
	        case "symbol":
	          node.removeAttribute(name);
	          return;
	        case "boolean":
	          var prefix$10 = name.toLowerCase().slice(0, 5);
	          if ("data-" !== prefix$10 && "aria-" !== prefix$10) {
	            node.removeAttribute(name);
	            return;
	          }
	      }
	      node.setAttribute(name, "" + value);
	    }
	}
	function setValueForKnownAttribute(node, name, value) {
	  if (null === value) node.removeAttribute(name);
	  else {
	    switch (typeof value) {
	      case "undefined":
	      case "function":
	      case "symbol":
	      case "boolean":
	        node.removeAttribute(name);
	        return;
	    }
	    node.setAttribute(name, "" + value);
	  }
	}
	function setValueForNamespacedAttribute(node, namespace, name, value) {
	  if (null === value) node.removeAttribute(name);
	  else {
	    switch (typeof value) {
	      case "undefined":
	      case "function":
	      case "symbol":
	      case "boolean":
	        node.removeAttribute(name);
	        return;
	    }
	    node.setAttributeNS(namespace, name, "" + value);
	  }
	}
	function getToStringValue(value) {
	  switch (typeof value) {
	    case "bigint":
	    case "boolean":
	    case "number":
	    case "string":
	    case "undefined":
	      return value;
	    case "object":
	      return value;
	    default:
	      return "";
	  }
	}
	function isCheckable(elem) {
	  var type = elem.type;
	  return (
	    (elem = elem.nodeName) &&
	    "input" === elem.toLowerCase() &&
	    ("checkbox" === type || "radio" === type)
	  );
	}
	function trackValueOnNode(node, valueField, currentValue) {
	  var descriptor = Object.getOwnPropertyDescriptor(
	    node.constructor.prototype,
	    valueField
	  );
	  if (
	    !node.hasOwnProperty(valueField) &&
	    "undefined" !== typeof descriptor &&
	    "function" === typeof descriptor.get &&
	    "function" === typeof descriptor.set
	  ) {
	    var get = descriptor.get,
	      set = descriptor.set;
	    Object.defineProperty(node, valueField, {
	      configurable: !0,
	      get: function () {
	        return get.call(this);
	      },
	      set: function (value) {
	        currentValue = "" + value;
	        set.call(this, value);
	      }
	    });
	    Object.defineProperty(node, valueField, {
	      enumerable: descriptor.enumerable
	    });
	    return {
	      getValue: function () {
	        return currentValue;
	      },
	      setValue: function (value) {
	        currentValue = "" + value;
	      },
	      stopTracking: function () {
	        node._valueTracker = null;
	        delete node[valueField];
	      }
	    };
	  }
	}
	function track(node) {
	  if (!node._valueTracker) {
	    var valueField = isCheckable(node) ? "checked" : "value";
	    node._valueTracker = trackValueOnNode(
	      node,
	      valueField,
	      "" + node[valueField]
	    );
	  }
	}
	function updateValueIfChanged(node) {
	  if (!node) return !1;
	  var tracker = node._valueTracker;
	  if (!tracker) return !0;
	  var lastValue = tracker.getValue();
	  var value = "";
	  node &&
	    (value = isCheckable(node)
	      ? node.checked
	        ? "true"
	        : "false"
	      : node.value);
	  node = value;
	  return node !== lastValue ? (tracker.setValue(node), !0) : !1;
	}
	function getActiveElement(doc) {
	  doc = doc || ("undefined" !== typeof document ? document : void 0);
	  if ("undefined" === typeof doc) return null;
	  try {
	    return doc.activeElement || doc.body;
	  } catch (e) {
	    return doc.body;
	  }
	}
	var escapeSelectorAttributeValueInsideDoubleQuotesRegex = /[\n"\\]/g;
	function escapeSelectorAttributeValueInsideDoubleQuotes(value) {
	  return value.replace(
	    escapeSelectorAttributeValueInsideDoubleQuotesRegex,
	    function (ch) {
	      return "\\" + ch.charCodeAt(0).toString(16) + " ";
	    }
	  );
	}
	function updateInput(
	  element,
	  value,
	  defaultValue,
	  lastDefaultValue,
	  checked,
	  defaultChecked,
	  type,
	  name
	) {
	  element.name = "";
	  null != type &&
	  "function" !== typeof type &&
	  "symbol" !== typeof type &&
	  "boolean" !== typeof type
	    ? (element.type = type)
	    : element.removeAttribute("type");
	  if (null != value)
	    if ("number" === type) {
	      if ((0 === value && "" === element.value) || element.value != value)
	        element.value = "" + getToStringValue(value);
	    } else
	      element.value !== "" + getToStringValue(value) &&
	        (element.value = "" + getToStringValue(value));
	  else
	    ("submit" !== type && "reset" !== type) || element.removeAttribute("value");
	  null != value
	    ? setDefaultValue(element, type, getToStringValue(value))
	    : null != defaultValue
	      ? setDefaultValue(element, type, getToStringValue(defaultValue))
	      : null != lastDefaultValue && element.removeAttribute("value");
	  null == checked &&
	    null != defaultChecked &&
	    (element.defaultChecked = !!defaultChecked);
	  null != checked &&
	    (element.checked =
	      checked && "function" !== typeof checked && "symbol" !== typeof checked);
	  null != name &&
	  "function" !== typeof name &&
	  "symbol" !== typeof name &&
	  "boolean" !== typeof name
	    ? (element.name = "" + getToStringValue(name))
	    : element.removeAttribute("name");
	}
	function initInput(
	  element,
	  value,
	  defaultValue,
	  checked,
	  defaultChecked,
	  type,
	  name,
	  isHydrating
	) {
	  null != type &&
	    "function" !== typeof type &&
	    "symbol" !== typeof type &&
	    "boolean" !== typeof type &&
	    (element.type = type);
	  if (null != value || null != defaultValue) {
	    if (
	      !(
	        ("submit" !== type && "reset" !== type) ||
	        (void 0 !== value && null !== value)
	      )
	    ) {
	      track(element);
	      return;
	    }
	    defaultValue =
	      null != defaultValue ? "" + getToStringValue(defaultValue) : "";
	    value = null != value ? "" + getToStringValue(value) : defaultValue;
	    isHydrating || value === element.value || (element.value = value);
	    element.defaultValue = value;
	  }
	  checked = null != checked ? checked : defaultChecked;
	  checked =
	    "function" !== typeof checked && "symbol" !== typeof checked && !!checked;
	  element.checked = isHydrating ? element.checked : !!checked;
	  element.defaultChecked = !!checked;
	  null != name &&
	    "function" !== typeof name &&
	    "symbol" !== typeof name &&
	    "boolean" !== typeof name &&
	    (element.name = name);
	  track(element);
	}
	function setDefaultValue(node, type, value) {
	  ("number" === type && getActiveElement(node.ownerDocument) === node) ||
	    node.defaultValue === "" + value ||
	    (node.defaultValue = "" + value);
	}
	function updateOptions(node, multiple, propValue, setDefaultSelected) {
	  node = node.options;
	  if (multiple) {
	    multiple = {};
	    for (var i = 0; i < propValue.length; i++)
	      multiple["$" + propValue[i]] = !0;
	    for (propValue = 0; propValue < node.length; propValue++)
	      (i = multiple.hasOwnProperty("$" + node[propValue].value)),
	        node[propValue].selected !== i && (node[propValue].selected = i),
	        i && setDefaultSelected && (node[propValue].defaultSelected = !0);
	  } else {
	    propValue = "" + getToStringValue(propValue);
	    multiple = null;
	    for (i = 0; i < node.length; i++) {
	      if (node[i].value === propValue) {
	        node[i].selected = !0;
	        setDefaultSelected && (node[i].defaultSelected = !0);
	        return;
	      }
	      null !== multiple || node[i].disabled || (multiple = node[i]);
	    }
	    null !== multiple && (multiple.selected = !0);
	  }
	}
	function updateTextarea(element, value, defaultValue) {
	  if (
	    null != value &&
	    ((value = "" + getToStringValue(value)),
	    value !== element.value && (element.value = value),
	    null == defaultValue)
	  ) {
	    element.defaultValue !== value && (element.defaultValue = value);
	    return;
	  }
	  element.defaultValue =
	    null != defaultValue ? "" + getToStringValue(defaultValue) : "";
	}
	function initTextarea(element, value, defaultValue, children) {
	  if (null == value) {
	    if (null != children) {
	      if (null != defaultValue) throw Error(formatProdErrorMessage(92));
	      if (isArrayImpl(children)) {
	        if (1 < children.length) throw Error(formatProdErrorMessage(93));
	        children = children[0];
	      }
	      defaultValue = children;
	    }
	    null == defaultValue && (defaultValue = "");
	    value = defaultValue;
	  }
	  defaultValue = getToStringValue(value);
	  element.defaultValue = defaultValue;
	  children = element.textContent;
	  children === defaultValue &&
	    "" !== children &&
	    null !== children &&
	    (element.value = children);
	  track(element);
	}
	function setTextContent(node, text) {
	  if (text) {
	    var firstChild = node.firstChild;
	    if (
	      firstChild &&
	      firstChild === node.lastChild &&
	      3 === firstChild.nodeType
	    ) {
	      firstChild.nodeValue = text;
	      return;
	    }
	  }
	  node.textContent = text;
	}
	var unitlessNumbers = new Set(
	  "animationIterationCount aspectRatio borderImageOutset borderImageSlice borderImageWidth boxFlex boxFlexGroup boxOrdinalGroup columnCount columns flex flexGrow flexPositive flexShrink flexNegative flexOrder gridArea gridRow gridRowEnd gridRowSpan gridRowStart gridColumn gridColumnEnd gridColumnSpan gridColumnStart fontWeight lineClamp lineHeight opacity order orphans scale tabSize widows zIndex zoom fillOpacity floodOpacity stopOpacity strokeDasharray strokeDashoffset strokeMiterlimit strokeOpacity strokeWidth MozAnimationIterationCount MozBoxFlex MozBoxFlexGroup MozLineClamp msAnimationIterationCount msFlex msZoom msFlexGrow msFlexNegative msFlexOrder msFlexPositive msFlexShrink msGridColumn msGridColumnSpan msGridRow msGridRowSpan WebkitAnimationIterationCount WebkitBoxFlex WebKitBoxFlexGroup WebkitBoxOrdinalGroup WebkitColumnCount WebkitColumns WebkitFlex WebkitFlexGrow WebkitFlexPositive WebkitFlexShrink WebkitLineClamp".split(
	    " "
	  )
	);
	function setValueForStyle(style, styleName, value) {
	  var isCustomProperty = 0 === styleName.indexOf("--");
	  null == value || "boolean" === typeof value || "" === value
	    ? isCustomProperty
	      ? style.setProperty(styleName, "")
	      : "float" === styleName
	        ? (style.cssFloat = "")
	        : (style[styleName] = "")
	    : isCustomProperty
	      ? style.setProperty(styleName, value)
	      : "number" !== typeof value ||
	          0 === value ||
	          unitlessNumbers.has(styleName)
	        ? "float" === styleName
	          ? (style.cssFloat = value)
	          : (style[styleName] = ("" + value).trim())
	        : (style[styleName] = value + "px");
	}
	function setValueForStyles(node, styles, prevStyles) {
	  if (null != styles && "object" !== typeof styles)
	    throw Error(formatProdErrorMessage(62));
	  node = node.style;
	  if (null != prevStyles) {
	    for (var styleName in prevStyles)
	      !prevStyles.hasOwnProperty(styleName) ||
	        (null != styles && styles.hasOwnProperty(styleName)) ||
	        (0 === styleName.indexOf("--")
	          ? node.setProperty(styleName, "")
	          : "float" === styleName
	            ? (node.cssFloat = "")
	            : (node[styleName] = ""));
	    for (var styleName$16 in styles)
	      (styleName = styles[styleName$16]),
	        styles.hasOwnProperty(styleName$16) &&
	          prevStyles[styleName$16] !== styleName &&
	          setValueForStyle(node, styleName$16, styleName);
	  } else
	    for (var styleName$17 in styles)
	      styles.hasOwnProperty(styleName$17) &&
	        setValueForStyle(node, styleName$17, styles[styleName$17]);
	}
	function isCustomElement(tagName) {
	  if (-1 === tagName.indexOf("-")) return !1;
	  switch (tagName) {
	    case "annotation-xml":
	    case "color-profile":
	    case "font-face":
	    case "font-face-src":
	    case "font-face-uri":
	    case "font-face-format":
	    case "font-face-name":
	    case "missing-glyph":
	      return !1;
	    default:
	      return !0;
	  }
	}
	var aliases = new Map([
	    ["acceptCharset", "accept-charset"],
	    ["htmlFor", "for"],
	    ["httpEquiv", "http-equiv"],
	    ["crossOrigin", "crossorigin"],
	    ["accentHeight", "accent-height"],
	    ["alignmentBaseline", "alignment-baseline"],
	    ["arabicForm", "arabic-form"],
	    ["baselineShift", "baseline-shift"],
	    ["capHeight", "cap-height"],
	    ["clipPath", "clip-path"],
	    ["clipRule", "clip-rule"],
	    ["colorInterpolation", "color-interpolation"],
	    ["colorInterpolationFilters", "color-interpolation-filters"],
	    ["colorProfile", "color-profile"],
	    ["colorRendering", "color-rendering"],
	    ["dominantBaseline", "dominant-baseline"],
	    ["enableBackground", "enable-background"],
	    ["fillOpacity", "fill-opacity"],
	    ["fillRule", "fill-rule"],
	    ["floodColor", "flood-color"],
	    ["floodOpacity", "flood-opacity"],
	    ["fontFamily", "font-family"],
	    ["fontSize", "font-size"],
	    ["fontSizeAdjust", "font-size-adjust"],
	    ["fontStretch", "font-stretch"],
	    ["fontStyle", "font-style"],
	    ["fontVariant", "font-variant"],
	    ["fontWeight", "font-weight"],
	    ["glyphName", "glyph-name"],
	    ["glyphOrientationHorizontal", "glyph-orientation-horizontal"],
	    ["glyphOrientationVertical", "glyph-orientation-vertical"],
	    ["horizAdvX", "horiz-adv-x"],
	    ["horizOriginX", "horiz-origin-x"],
	    ["imageRendering", "image-rendering"],
	    ["letterSpacing", "letter-spacing"],
	    ["lightingColor", "lighting-color"],
	    ["markerEnd", "marker-end"],
	    ["markerMid", "marker-mid"],
	    ["markerStart", "marker-start"],
	    ["overlinePosition", "overline-position"],
	    ["overlineThickness", "overline-thickness"],
	    ["paintOrder", "paint-order"],
	    ["panose-1", "panose-1"],
	    ["pointerEvents", "pointer-events"],
	    ["renderingIntent", "rendering-intent"],
	    ["shapeRendering", "shape-rendering"],
	    ["stopColor", "stop-color"],
	    ["stopOpacity", "stop-opacity"],
	    ["strikethroughPosition", "strikethrough-position"],
	    ["strikethroughThickness", "strikethrough-thickness"],
	    ["strokeDasharray", "stroke-dasharray"],
	    ["strokeDashoffset", "stroke-dashoffset"],
	    ["strokeLinecap", "stroke-linecap"],
	    ["strokeLinejoin", "stroke-linejoin"],
	    ["strokeMiterlimit", "stroke-miterlimit"],
	    ["strokeOpacity", "stroke-opacity"],
	    ["strokeWidth", "stroke-width"],
	    ["textAnchor", "text-anchor"],
	    ["textDecoration", "text-decoration"],
	    ["textRendering", "text-rendering"],
	    ["transformOrigin", "transform-origin"],
	    ["underlinePosition", "underline-position"],
	    ["underlineThickness", "underline-thickness"],
	    ["unicodeBidi", "unicode-bidi"],
	    ["unicodeRange", "unicode-range"],
	    ["unitsPerEm", "units-per-em"],
	    ["vAlphabetic", "v-alphabetic"],
	    ["vHanging", "v-hanging"],
	    ["vIdeographic", "v-ideographic"],
	    ["vMathematical", "v-mathematical"],
	    ["vectorEffect", "vector-effect"],
	    ["vertAdvY", "vert-adv-y"],
	    ["vertOriginX", "vert-origin-x"],
	    ["vertOriginY", "vert-origin-y"],
	    ["wordSpacing", "word-spacing"],
	    ["writingMode", "writing-mode"],
	    ["xmlnsXlink", "xmlns:xlink"],
	    ["xHeight", "x-height"]
	  ]),
	  isJavaScriptProtocol =
	    /^[\u0000-\u001F ]*j[\r\n\t]*a[\r\n\t]*v[\r\n\t]*a[\r\n\t]*s[\r\n\t]*c[\r\n\t]*r[\r\n\t]*i[\r\n\t]*p[\r\n\t]*t[\r\n\t]*:/i;
	function sanitizeURL(url) {
	  return isJavaScriptProtocol.test("" + url)
	    ? "javascript:throw new Error('React has blocked a javascript: URL as a security precaution.')"
	    : url;
	}
	function noop$1() {}
	var currentReplayingEvent = null;
	function getEventTarget(nativeEvent) {
	  nativeEvent = nativeEvent.target || nativeEvent.srcElement || window;
	  nativeEvent.correspondingUseElement &&
	    (nativeEvent = nativeEvent.correspondingUseElement);
	  return 3 === nativeEvent.nodeType ? nativeEvent.parentNode : nativeEvent;
	}
	var restoreTarget = null,
	  restoreQueue = null;
	function restoreStateOfTarget(target) {
	  var internalInstance = getInstanceFromNode(target);
	  if (internalInstance && (target = internalInstance.stateNode)) {
	    var props = target[internalPropsKey] || null;
	    a: switch (((target = internalInstance.stateNode), internalInstance.type)) {
	      case "input":
	        updateInput(
	          target,
	          props.value,
	          props.defaultValue,
	          props.defaultValue,
	          props.checked,
	          props.defaultChecked,
	          props.type,
	          props.name
	        );
	        internalInstance = props.name;
	        if ("radio" === props.type && null != internalInstance) {
	          for (props = target; props.parentNode; ) props = props.parentNode;
	          props = props.querySelectorAll(
	            'input[name="' +
	              escapeSelectorAttributeValueInsideDoubleQuotes(
	                "" + internalInstance
	              ) +
	              '"][type="radio"]'
	          );
	          for (
	            internalInstance = 0;
	            internalInstance < props.length;
	            internalInstance++
	          ) {
	            var otherNode = props[internalInstance];
	            if (otherNode !== target && otherNode.form === target.form) {
	              var otherProps = otherNode[internalPropsKey] || null;
	              if (!otherProps) throw Error(formatProdErrorMessage(90));
	              updateInput(
	                otherNode,
	                otherProps.value,
	                otherProps.defaultValue,
	                otherProps.defaultValue,
	                otherProps.checked,
	                otherProps.defaultChecked,
	                otherProps.type,
	                otherProps.name
	              );
	            }
	          }
	          for (
	            internalInstance = 0;
	            internalInstance < props.length;
	            internalInstance++
	          )
	            (otherNode = props[internalInstance]),
	              otherNode.form === target.form && updateValueIfChanged(otherNode);
	        }
	        break a;
	      case "textarea":
	        updateTextarea(target, props.value, props.defaultValue);
	        break a;
	      case "select":
	        (internalInstance = props.value),
	          null != internalInstance &&
	            updateOptions(target, !!props.multiple, internalInstance, !1);
	    }
	  }
	}
	var isInsideEventHandler = !1;
	function batchedUpdates$1(fn, a, b) {
	  if (isInsideEventHandler) return fn(a, b);
	  isInsideEventHandler = !0;
	  try {
	    var JSCompiler_inline_result = fn(a);
	    return JSCompiler_inline_result;
	  } finally {
	    if (
	      ((isInsideEventHandler = !1),
	      null !== restoreTarget || null !== restoreQueue)
	    )
	      if (
	        (flushSyncWork$1(),
	        restoreTarget &&
	          ((a = restoreTarget),
	          (fn = restoreQueue),
	          (restoreQueue = restoreTarget = null),
	          restoreStateOfTarget(a),
	          fn))
	      )
	        for (a = 0; a < fn.length; a++) restoreStateOfTarget(fn[a]);
	  }
	}
	function getListener(inst, registrationName) {
	  var stateNode = inst.stateNode;
	  if (null === stateNode) return null;
	  var props = stateNode[internalPropsKey] || null;
	  if (null === props) return null;
	  stateNode = props[registrationName];
	  a: switch (registrationName) {
	    case "onClick":
	    case "onClickCapture":
	    case "onDoubleClick":
	    case "onDoubleClickCapture":
	    case "onMouseDown":
	    case "onMouseDownCapture":
	    case "onMouseMove":
	    case "onMouseMoveCapture":
	    case "onMouseUp":
	    case "onMouseUpCapture":
	    case "onMouseEnter":
	      (props = !props.disabled) ||
	        ((inst = inst.type),
	        (props = !(
	          "button" === inst ||
	          "input" === inst ||
	          "select" === inst ||
	          "textarea" === inst
	        )));
	      inst = !props;
	      break a;
	    default:
	      inst = !1;
	  }
	  if (inst) return null;
	  if (stateNode && "function" !== typeof stateNode)
	    throw Error(
	      formatProdErrorMessage(231, registrationName, typeof stateNode)
	    );
	  return stateNode;
	}
	var canUseDOM = !(
	    "undefined" === typeof window ||
	    "undefined" === typeof window.document ||
	    "undefined" === typeof window.document.createElement
	  ),
	  passiveBrowserEventsSupported = !1;
	if (canUseDOM)
	  try {
	    var options = {};
	    Object.defineProperty(options, "passive", {
	      get: function () {
	        passiveBrowserEventsSupported = !0;
	      }
	    });
	    window.addEventListener("test", options, options);
	    window.removeEventListener("test", options, options);
	  } catch (e) {
	    passiveBrowserEventsSupported = !1;
	  }
	var root = null,
	  startText = null,
	  fallbackText = null;
	function getData() {
	  if (fallbackText) return fallbackText;
	  var start,
	    startValue = startText,
	    startLength = startValue.length,
	    end,
	    endValue = "value" in root ? root.value : root.textContent,
	    endLength = endValue.length;
	  for (
	    start = 0;
	    start < startLength && startValue[start] === endValue[start];
	    start++
	  );
	  var minEnd = startLength - start;
	  for (
	    end = 1;
	    end <= minEnd &&
	    startValue[startLength - end] === endValue[endLength - end];
	    end++
	  );
	  return (fallbackText = endValue.slice(start, 1 < end ? 1 - end : void 0));
	}
	function getEventCharCode(nativeEvent) {
	  var keyCode = nativeEvent.keyCode;
	  "charCode" in nativeEvent
	    ? ((nativeEvent = nativeEvent.charCode),
	      0 === nativeEvent && 13 === keyCode && (nativeEvent = 13))
	    : (nativeEvent = keyCode);
	  10 === nativeEvent && (nativeEvent = 13);
	  return 32 <= nativeEvent || 13 === nativeEvent ? nativeEvent : 0;
	}
	function functionThatReturnsTrue() {
	  return !0;
	}
	function functionThatReturnsFalse() {
	  return !1;
	}
	function createSyntheticEvent(Interface) {
	  function SyntheticBaseEvent(
	    reactName,
	    reactEventType,
	    targetInst,
	    nativeEvent,
	    nativeEventTarget
	  ) {
	    this._reactName = reactName;
	    this._targetInst = targetInst;
	    this.type = reactEventType;
	    this.nativeEvent = nativeEvent;
	    this.target = nativeEventTarget;
	    this.currentTarget = null;
	    for (var propName in Interface)
	      Interface.hasOwnProperty(propName) &&
	        ((reactName = Interface[propName]),
	        (this[propName] = reactName
	          ? reactName(nativeEvent)
	          : nativeEvent[propName]));
	    this.isDefaultPrevented = (
	      null != nativeEvent.defaultPrevented
	        ? nativeEvent.defaultPrevented
	        : !1 === nativeEvent.returnValue
	    )
	      ? functionThatReturnsTrue
	      : functionThatReturnsFalse;
	    this.isPropagationStopped = functionThatReturnsFalse;
	    return this;
	  }
	  assign(SyntheticBaseEvent.prototype, {
	    preventDefault: function () {
	      this.defaultPrevented = !0;
	      var event = this.nativeEvent;
	      event &&
	        (event.preventDefault
	          ? event.preventDefault()
	          : "unknown" !== typeof event.returnValue && (event.returnValue = !1),
	        (this.isDefaultPrevented = functionThatReturnsTrue));
	    },
	    stopPropagation: function () {
	      var event = this.nativeEvent;
	      event &&
	        (event.stopPropagation
	          ? event.stopPropagation()
	          : "unknown" !== typeof event.cancelBubble &&
	            (event.cancelBubble = !0),
	        (this.isPropagationStopped = functionThatReturnsTrue));
	    },
	    persist: function () {},
	    isPersistent: functionThatReturnsTrue
	  });
	  return SyntheticBaseEvent;
	}
	var EventInterface = {
	    eventPhase: 0,
	    bubbles: 0,
	    cancelable: 0,
	    timeStamp: function (event) {
	      return event.timeStamp || Date.now();
	    },
	    defaultPrevented: 0,
	    isTrusted: 0
	  },
	  SyntheticEvent = createSyntheticEvent(EventInterface),
	  UIEventInterface = assign({}, EventInterface, { view: 0, detail: 0 }),
	  SyntheticUIEvent = createSyntheticEvent(UIEventInterface),
	  lastMovementX,
	  lastMovementY,
	  lastMouseEvent,
	  MouseEventInterface = assign({}, UIEventInterface, {
	    screenX: 0,
	    screenY: 0,
	    clientX: 0,
	    clientY: 0,
	    pageX: 0,
	    pageY: 0,
	    ctrlKey: 0,
	    shiftKey: 0,
	    altKey: 0,
	    metaKey: 0,
	    getModifierState: getEventModifierState,
	    button: 0,
	    buttons: 0,
	    relatedTarget: function (event) {
	      return void 0 === event.relatedTarget
	        ? event.fromElement === event.srcElement
	          ? event.toElement
	          : event.fromElement
	        : event.relatedTarget;
	    },
	    movementX: function (event) {
	      if ("movementX" in event) return event.movementX;
	      event !== lastMouseEvent &&
	        (lastMouseEvent && "mousemove" === event.type
	          ? ((lastMovementX = event.screenX - lastMouseEvent.screenX),
	            (lastMovementY = event.screenY - lastMouseEvent.screenY))
	          : (lastMovementY = lastMovementX = 0),
	        (lastMouseEvent = event));
	      return lastMovementX;
	    },
	    movementY: function (event) {
	      return "movementY" in event ? event.movementY : lastMovementY;
	    }
	  }),
	  SyntheticMouseEvent = createSyntheticEvent(MouseEventInterface),
	  DragEventInterface = assign({}, MouseEventInterface, { dataTransfer: 0 }),
	  SyntheticDragEvent = createSyntheticEvent(DragEventInterface),
	  FocusEventInterface = assign({}, UIEventInterface, { relatedTarget: 0 }),
	  SyntheticFocusEvent = createSyntheticEvent(FocusEventInterface),
	  AnimationEventInterface = assign({}, EventInterface, {
	    animationName: 0,
	    elapsedTime: 0,
	    pseudoElement: 0
	  }),
	  SyntheticAnimationEvent = createSyntheticEvent(AnimationEventInterface),
	  ClipboardEventInterface = assign({}, EventInterface, {
	    clipboardData: function (event) {
	      return "clipboardData" in event
	        ? event.clipboardData
	        : window.clipboardData;
	    }
	  }),
	  SyntheticClipboardEvent = createSyntheticEvent(ClipboardEventInterface),
	  CompositionEventInterface = assign({}, EventInterface, { data: 0 }),
	  SyntheticCompositionEvent = createSyntheticEvent(CompositionEventInterface),
	  normalizeKey = {
	    Esc: "Escape",
	    Spacebar: " ",
	    Left: "ArrowLeft",
	    Up: "ArrowUp",
	    Right: "ArrowRight",
	    Down: "ArrowDown",
	    Del: "Delete",
	    Win: "OS",
	    Menu: "ContextMenu",
	    Apps: "ContextMenu",
	    Scroll: "ScrollLock",
	    MozPrintableKey: "Unidentified"
	  },
	  translateToKey = {
	    8: "Backspace",
	    9: "Tab",
	    12: "Clear",
	    13: "Enter",
	    16: "Shift",
	    17: "Control",
	    18: "Alt",
	    19: "Pause",
	    20: "CapsLock",
	    27: "Escape",
	    32: " ",
	    33: "PageUp",
	    34: "PageDown",
	    35: "End",
	    36: "Home",
	    37: "ArrowLeft",
	    38: "ArrowUp",
	    39: "ArrowRight",
	    40: "ArrowDown",
	    45: "Insert",
	    46: "Delete",
	    112: "F1",
	    113: "F2",
	    114: "F3",
	    115: "F4",
	    116: "F5",
	    117: "F6",
	    118: "F7",
	    119: "F8",
	    120: "F9",
	    121: "F10",
	    122: "F11",
	    123: "F12",
	    144: "NumLock",
	    145: "ScrollLock",
	    224: "Meta"
	  },
	  modifierKeyToProp = {
	    Alt: "altKey",
	    Control: "ctrlKey",
	    Meta: "metaKey",
	    Shift: "shiftKey"
	  };
	function modifierStateGetter(keyArg) {
	  var nativeEvent = this.nativeEvent;
	  return nativeEvent.getModifierState
	    ? nativeEvent.getModifierState(keyArg)
	    : (keyArg = modifierKeyToProp[keyArg])
	      ? !!nativeEvent[keyArg]
	      : !1;
	}
	function getEventModifierState() {
	  return modifierStateGetter;
	}
	var KeyboardEventInterface = assign({}, UIEventInterface, {
	    key: function (nativeEvent) {
	      if (nativeEvent.key) {
	        var key = normalizeKey[nativeEvent.key] || nativeEvent.key;
	        if ("Unidentified" !== key) return key;
	      }
	      return "keypress" === nativeEvent.type
	        ? ((nativeEvent = getEventCharCode(nativeEvent)),
	          13 === nativeEvent ? "Enter" : String.fromCharCode(nativeEvent))
	        : "keydown" === nativeEvent.type || "keyup" === nativeEvent.type
	          ? translateToKey[nativeEvent.keyCode] || "Unidentified"
	          : "";
	    },
	    code: 0,
	    location: 0,
	    ctrlKey: 0,
	    shiftKey: 0,
	    altKey: 0,
	    metaKey: 0,
	    repeat: 0,
	    locale: 0,
	    getModifierState: getEventModifierState,
	    charCode: function (event) {
	      return "keypress" === event.type ? getEventCharCode(event) : 0;
	    },
	    keyCode: function (event) {
	      return "keydown" === event.type || "keyup" === event.type
	        ? event.keyCode
	        : 0;
	    },
	    which: function (event) {
	      return "keypress" === event.type
	        ? getEventCharCode(event)
	        : "keydown" === event.type || "keyup" === event.type
	          ? event.keyCode
	          : 0;
	    }
	  }),
	  SyntheticKeyboardEvent = createSyntheticEvent(KeyboardEventInterface),
	  PointerEventInterface = assign({}, MouseEventInterface, {
	    pointerId: 0,
	    width: 0,
	    height: 0,
	    pressure: 0,
	    tangentialPressure: 0,
	    tiltX: 0,
	    tiltY: 0,
	    twist: 0,
	    pointerType: 0,
	    isPrimary: 0
	  }),
	  SyntheticPointerEvent = createSyntheticEvent(PointerEventInterface),
	  TouchEventInterface = assign({}, UIEventInterface, {
	    touches: 0,
	    targetTouches: 0,
	    changedTouches: 0,
	    altKey: 0,
	    metaKey: 0,
	    ctrlKey: 0,
	    shiftKey: 0,
	    getModifierState: getEventModifierState
	  }),
	  SyntheticTouchEvent = createSyntheticEvent(TouchEventInterface),
	  TransitionEventInterface = assign({}, EventInterface, {
	    propertyName: 0,
	    elapsedTime: 0,
	    pseudoElement: 0
	  }),
	  SyntheticTransitionEvent = createSyntheticEvent(TransitionEventInterface),
	  WheelEventInterface = assign({}, MouseEventInterface, {
	    deltaX: function (event) {
	      return "deltaX" in event
	        ? event.deltaX
	        : "wheelDeltaX" in event
	          ? -event.wheelDeltaX
	          : 0;
	    },
	    deltaY: function (event) {
	      return "deltaY" in event
	        ? event.deltaY
	        : "wheelDeltaY" in event
	          ? -event.wheelDeltaY
	          : "wheelDelta" in event
	            ? -event.wheelDelta
	            : 0;
	    },
	    deltaZ: 0,
	    deltaMode: 0
	  }),
	  SyntheticWheelEvent = createSyntheticEvent(WheelEventInterface),
	  ToggleEventInterface = assign({}, EventInterface, {
	    newState: 0,
	    oldState: 0
	  }),
	  SyntheticToggleEvent = createSyntheticEvent(ToggleEventInterface),
	  END_KEYCODES = [9, 13, 27, 32],
	  canUseCompositionEvent = canUseDOM && "CompositionEvent" in window,
	  documentMode = null;
	canUseDOM &&
	  "documentMode" in document &&
	  (documentMode = document.documentMode);
	var canUseTextInputEvent = canUseDOM && "TextEvent" in window && !documentMode,
	  useFallbackCompositionData =
	    canUseDOM &&
	    (!canUseCompositionEvent ||
	      (documentMode && 8 < documentMode && 11 >= documentMode)),
	  SPACEBAR_CHAR = String.fromCharCode(32),
	  hasSpaceKeypress = !1;
	function isFallbackCompositionEnd(domEventName, nativeEvent) {
	  switch (domEventName) {
	    case "keyup":
	      return -1 !== END_KEYCODES.indexOf(nativeEvent.keyCode);
	    case "keydown":
	      return 229 !== nativeEvent.keyCode;
	    case "keypress":
	    case "mousedown":
	    case "focusout":
	      return !0;
	    default:
	      return !1;
	  }
	}
	function getDataFromCustomEvent(nativeEvent) {
	  nativeEvent = nativeEvent.detail;
	  return "object" === typeof nativeEvent && "data" in nativeEvent
	    ? nativeEvent.data
	    : null;
	}
	var isComposing = !1;
	function getNativeBeforeInputChars(domEventName, nativeEvent) {
	  switch (domEventName) {
	    case "compositionend":
	      return getDataFromCustomEvent(nativeEvent);
	    case "keypress":
	      if (32 !== nativeEvent.which) return null;
	      hasSpaceKeypress = !0;
	      return SPACEBAR_CHAR;
	    case "textInput":
	      return (
	        (domEventName = nativeEvent.data),
	        domEventName === SPACEBAR_CHAR && hasSpaceKeypress ? null : domEventName
	      );
	    default:
	      return null;
	  }
	}
	function getFallbackBeforeInputChars(domEventName, nativeEvent) {
	  if (isComposing)
	    return "compositionend" === domEventName ||
	      (!canUseCompositionEvent &&
	        isFallbackCompositionEnd(domEventName, nativeEvent))
	      ? ((domEventName = getData()),
	        (fallbackText = startText = root = null),
	        (isComposing = !1),
	        domEventName)
	      : null;
	  switch (domEventName) {
	    case "paste":
	      return null;
	    case "keypress":
	      if (
	        !(nativeEvent.ctrlKey || nativeEvent.altKey || nativeEvent.metaKey) ||
	        (nativeEvent.ctrlKey && nativeEvent.altKey)
	      ) {
	        if (nativeEvent.char && 1 < nativeEvent.char.length)
	          return nativeEvent.char;
	        if (nativeEvent.which) return String.fromCharCode(nativeEvent.which);
	      }
	      return null;
	    case "compositionend":
	      return useFallbackCompositionData && "ko" !== nativeEvent.locale
	        ? null
	        : nativeEvent.data;
	    default:
	      return null;
	  }
	}
	var supportedInputTypes = {
	  color: !0,
	  date: !0,
	  datetime: !0,
	  "datetime-local": !0,
	  email: !0,
	  month: !0,
	  number: !0,
	  password: !0,
	  range: !0,
	  search: !0,
	  tel: !0,
	  text: !0,
	  time: !0,
	  url: !0,
	  week: !0
	};
	function isTextInputElement(elem) {
	  var nodeName = elem && elem.nodeName && elem.nodeName.toLowerCase();
	  return "input" === nodeName
	    ? !!supportedInputTypes[elem.type]
	    : "textarea" === nodeName
	      ? !0
	      : !1;
	}
	function createAndAccumulateChangeEvent(
	  dispatchQueue,
	  inst,
	  nativeEvent,
	  target
	) {
	  restoreTarget
	    ? restoreQueue
	      ? restoreQueue.push(target)
	      : (restoreQueue = [target])
	    : (restoreTarget = target);
	  inst = accumulateTwoPhaseListeners(inst, "onChange");
	  0 < inst.length &&
	    ((nativeEvent = new SyntheticEvent(
	      "onChange",
	      "change",
	      null,
	      nativeEvent,
	      target
	    )),
	    dispatchQueue.push({ event: nativeEvent, listeners: inst }));
	}
	var activeElement$1 = null,
	  activeElementInst$1 = null;
	function runEventInBatch(dispatchQueue) {
	  processDispatchQueue(dispatchQueue, 0);
	}
	function getInstIfValueChanged(targetInst) {
	  var targetNode = getNodeFromInstance(targetInst);
	  if (updateValueIfChanged(targetNode)) return targetInst;
	}
	function getTargetInstForChangeEvent(domEventName, targetInst) {
	  if ("change" === domEventName) return targetInst;
	}
	var isInputEventSupported = !1;
	if (canUseDOM) {
	  var JSCompiler_inline_result$jscomp$286;
	  if (canUseDOM) {
	    var isSupported$jscomp$inline_427 = "oninput" in document;
	    if (!isSupported$jscomp$inline_427) {
	      var element$jscomp$inline_428 = document.createElement("div");
	      element$jscomp$inline_428.setAttribute("oninput", "return;");
	      isSupported$jscomp$inline_427 =
	        "function" === typeof element$jscomp$inline_428.oninput;
	    }
	    JSCompiler_inline_result$jscomp$286 = isSupported$jscomp$inline_427;
	  } else JSCompiler_inline_result$jscomp$286 = !1;
	  isInputEventSupported =
	    JSCompiler_inline_result$jscomp$286 &&
	    (!document.documentMode || 9 < document.documentMode);
	}
	function stopWatchingForValueChange() {
	  activeElement$1 &&
	    (activeElement$1.detachEvent("onpropertychange", handlePropertyChange),
	    (activeElementInst$1 = activeElement$1 = null));
	}
	function handlePropertyChange(nativeEvent) {
	  if (
	    "value" === nativeEvent.propertyName &&
	    getInstIfValueChanged(activeElementInst$1)
	  ) {
	    var dispatchQueue = [];
	    createAndAccumulateChangeEvent(
	      dispatchQueue,
	      activeElementInst$1,
	      nativeEvent,
	      getEventTarget(nativeEvent)
	    );
	    batchedUpdates$1(runEventInBatch, dispatchQueue);
	  }
	}
	function handleEventsForInputEventPolyfill(domEventName, target, targetInst) {
	  "focusin" === domEventName
	    ? (stopWatchingForValueChange(),
	      (activeElement$1 = target),
	      (activeElementInst$1 = targetInst),
	      activeElement$1.attachEvent("onpropertychange", handlePropertyChange))
	    : "focusout" === domEventName && stopWatchingForValueChange();
	}
	function getTargetInstForInputEventPolyfill(domEventName) {
	  if (
	    "selectionchange" === domEventName ||
	    "keyup" === domEventName ||
	    "keydown" === domEventName
	  )
	    return getInstIfValueChanged(activeElementInst$1);
	}
	function getTargetInstForClickEvent(domEventName, targetInst) {
	  if ("click" === domEventName) return getInstIfValueChanged(targetInst);
	}
	function getTargetInstForInputOrChangeEvent(domEventName, targetInst) {
	  if ("input" === domEventName || "change" === domEventName)
	    return getInstIfValueChanged(targetInst);
	}
	function is(x, y) {
	  return (x === y && (0 !== x || 1 / x === 1 / y)) || (x !== x && y !== y);
	}
	var objectIs = "function" === typeof Object.is ? Object.is : is;
	function shallowEqual(objA, objB) {
	  if (objectIs(objA, objB)) return !0;
	  if (
	    "object" !== typeof objA ||
	    null === objA ||
	    "object" !== typeof objB ||
	    null === objB
	  )
	    return !1;
	  var keysA = Object.keys(objA),
	    keysB = Object.keys(objB);
	  if (keysA.length !== keysB.length) return !1;
	  for (keysB = 0; keysB < keysA.length; keysB++) {
	    var currentKey = keysA[keysB];
	    if (
	      !hasOwnProperty.call(objB, currentKey) ||
	      !objectIs(objA[currentKey], objB[currentKey])
	    )
	      return !1;
	  }
	  return !0;
	}
	function getLeafNode(node) {
	  for (; node && node.firstChild; ) node = node.firstChild;
	  return node;
	}
	function getNodeForCharacterOffset(root, offset) {
	  var node = getLeafNode(root);
	  root = 0;
	  for (var nodeEnd; node; ) {
	    if (3 === node.nodeType) {
	      nodeEnd = root + node.textContent.length;
	      if (root <= offset && nodeEnd >= offset)
	        return { node: node, offset: offset - root };
	      root = nodeEnd;
	    }
	    a: {
	      for (; node; ) {
	        if (node.nextSibling) {
	          node = node.nextSibling;
	          break a;
	        }
	        node = node.parentNode;
	      }
	      node = void 0;
	    }
	    node = getLeafNode(node);
	  }
	}
	function containsNode(outerNode, innerNode) {
	  return outerNode && innerNode
	    ? outerNode === innerNode
	      ? !0
	      : outerNode && 3 === outerNode.nodeType
	        ? !1
	        : innerNode && 3 === innerNode.nodeType
	          ? containsNode(outerNode, innerNode.parentNode)
	          : "contains" in outerNode
	            ? outerNode.contains(innerNode)
	            : outerNode.compareDocumentPosition
	              ? !!(outerNode.compareDocumentPosition(innerNode) & 16)
	              : !1
	    : !1;
	}
	function getActiveElementDeep(containerInfo) {
	  containerInfo =
	    null != containerInfo &&
	    null != containerInfo.ownerDocument &&
	    null != containerInfo.ownerDocument.defaultView
	      ? containerInfo.ownerDocument.defaultView
	      : window;
	  for (
	    var element = getActiveElement(containerInfo.document);
	    element instanceof containerInfo.HTMLIFrameElement;

	  ) {
	    try {
	      var JSCompiler_inline_result =
	        "string" === typeof element.contentWindow.location.href;
	    } catch (err) {
	      JSCompiler_inline_result = !1;
	    }
	    if (JSCompiler_inline_result) containerInfo = element.contentWindow;
	    else break;
	    element = getActiveElement(containerInfo.document);
	  }
	  return element;
	}
	function hasSelectionCapabilities(elem) {
	  var nodeName = elem && elem.nodeName && elem.nodeName.toLowerCase();
	  return (
	    nodeName &&
	    (("input" === nodeName &&
	      ("text" === elem.type ||
	        "search" === elem.type ||
	        "tel" === elem.type ||
	        "url" === elem.type ||
	        "password" === elem.type)) ||
	      "textarea" === nodeName ||
	      "true" === elem.contentEditable)
	  );
	}
	var skipSelectionChangeEvent =
	    canUseDOM && "documentMode" in document && 11 >= document.documentMode,
	  activeElement = null,
	  activeElementInst = null,
	  lastSelection = null,
	  mouseDown = !1;
	function constructSelectEvent(dispatchQueue, nativeEvent, nativeEventTarget) {
	  var doc =
	    nativeEventTarget.window === nativeEventTarget
	      ? nativeEventTarget.document
	      : 9 === nativeEventTarget.nodeType
	        ? nativeEventTarget
	        : nativeEventTarget.ownerDocument;
	  mouseDown ||
	    null == activeElement ||
	    activeElement !== getActiveElement(doc) ||
	    ((doc = activeElement),
	    "selectionStart" in doc && hasSelectionCapabilities(doc)
	      ? (doc = { start: doc.selectionStart, end: doc.selectionEnd })
	      : ((doc = (
	          (doc.ownerDocument && doc.ownerDocument.defaultView) ||
	          window
	        ).getSelection()),
	        (doc = {
	          anchorNode: doc.anchorNode,
	          anchorOffset: doc.anchorOffset,
	          focusNode: doc.focusNode,
	          focusOffset: doc.focusOffset
	        })),
	    (lastSelection && shallowEqual(lastSelection, doc)) ||
	      ((lastSelection = doc),
	      (doc = accumulateTwoPhaseListeners(activeElementInst, "onSelect")),
	      0 < doc.length &&
	        ((nativeEvent = new SyntheticEvent(
	          "onSelect",
	          "select",
	          null,
	          nativeEvent,
	          nativeEventTarget
	        )),
	        dispatchQueue.push({ event: nativeEvent, listeners: doc }),
	        (nativeEvent.target = activeElement))));
	}
	function makePrefixMap(styleProp, eventName) {
	  var prefixes = {};
	  prefixes[styleProp.toLowerCase()] = eventName.toLowerCase();
	  prefixes["Webkit" + styleProp] = "webkit" + eventName;
	  prefixes["Moz" + styleProp] = "moz" + eventName;
	  return prefixes;
	}
	var vendorPrefixes = {
	    animationend: makePrefixMap("Animation", "AnimationEnd"),
	    animationiteration: makePrefixMap("Animation", "AnimationIteration"),
	    animationstart: makePrefixMap("Animation", "AnimationStart"),
	    transitionrun: makePrefixMap("Transition", "TransitionRun"),
	    transitionstart: makePrefixMap("Transition", "TransitionStart"),
	    transitioncancel: makePrefixMap("Transition", "TransitionCancel"),
	    transitionend: makePrefixMap("Transition", "TransitionEnd")
	  },
	  prefixedEventNames = {},
	  style = {};
	canUseDOM &&
	  ((style = document.createElement("div").style),
	  "AnimationEvent" in window ||
	    (delete vendorPrefixes.animationend.animation,
	    delete vendorPrefixes.animationiteration.animation,
	    delete vendorPrefixes.animationstart.animation),
	  "TransitionEvent" in window ||
	    delete vendorPrefixes.transitionend.transition);
	function getVendorPrefixedEventName(eventName) {
	  if (prefixedEventNames[eventName]) return prefixedEventNames[eventName];
	  if (!vendorPrefixes[eventName]) return eventName;
	  var prefixMap = vendorPrefixes[eventName],
	    styleProp;
	  for (styleProp in prefixMap)
	    if (prefixMap.hasOwnProperty(styleProp) && styleProp in style)
	      return (prefixedEventNames[eventName] = prefixMap[styleProp]);
	  return eventName;
	}
	var ANIMATION_END = getVendorPrefixedEventName("animationend"),
	  ANIMATION_ITERATION = getVendorPrefixedEventName("animationiteration"),
	  ANIMATION_START = getVendorPrefixedEventName("animationstart"),
	  TRANSITION_RUN = getVendorPrefixedEventName("transitionrun"),
	  TRANSITION_START = getVendorPrefixedEventName("transitionstart"),
	  TRANSITION_CANCEL = getVendorPrefixedEventName("transitioncancel"),
	  TRANSITION_END = getVendorPrefixedEventName("transitionend"),
	  topLevelEventsToReactNames = new Map(),
	  simpleEventPluginEvents =
	    "abort auxClick beforeToggle cancel canPlay canPlayThrough click close contextMenu copy cut drag dragEnd dragEnter dragExit dragLeave dragOver dragStart drop durationChange emptied encrypted ended error gotPointerCapture input invalid keyDown keyPress keyUp load loadedData loadedMetadata loadStart lostPointerCapture mouseDown mouseMove mouseOut mouseOver mouseUp paste pause play playing pointerCancel pointerDown pointerMove pointerOut pointerOver pointerUp progress rateChange reset resize seeked seeking stalled submit suspend timeUpdate touchCancel touchEnd touchStart volumeChange scroll toggle touchMove waiting wheel".split(
	      " "
	    );
	simpleEventPluginEvents.push("scrollEnd");
	function registerSimpleEvent(domEventName, reactName) {
	  topLevelEventsToReactNames.set(domEventName, reactName);
	  registerTwoPhaseEvent(reactName, [domEventName]);
	}
	var reportGlobalError =
	    "function" === typeof reportError
	      ? reportError
	      : function (error) {
	          if (
	            "object" === typeof window &&
	            "function" === typeof window.ErrorEvent
	          ) {
	            var event = new window.ErrorEvent("error", {
	              bubbles: !0,
	              cancelable: !0,
	              message:
	                "object" === typeof error &&
	                null !== error &&
	                "string" === typeof error.message
	                  ? String(error.message)
	                  : String(error),
	              error: error
	            });
	            if (!window.dispatchEvent(event)) return;
	          } else if (
	            "object" === typeof process &&
	            "function" === typeof process.emit
	          ) {
	            process.emit("uncaughtException", error);
	            return;
	          }
	          console.error(error);
	        },
	  concurrentQueues = [],
	  concurrentQueuesIndex = 0,
	  concurrentlyUpdatedLanes = 0;
	function finishQueueingConcurrentUpdates() {
	  for (
	    var endIndex = concurrentQueuesIndex,
	      i = (concurrentlyUpdatedLanes = concurrentQueuesIndex = 0);
	    i < endIndex;

	  ) {
	    var fiber = concurrentQueues[i];
	    concurrentQueues[i++] = null;
	    var queue = concurrentQueues[i];
	    concurrentQueues[i++] = null;
	    var update = concurrentQueues[i];
	    concurrentQueues[i++] = null;
	    var lane = concurrentQueues[i];
	    concurrentQueues[i++] = null;
	    if (null !== queue && null !== update) {
	      var pending = queue.pending;
	      null === pending
	        ? (update.next = update)
	        : ((update.next = pending.next), (pending.next = update));
	      queue.pending = update;
	    }
	    0 !== lane && markUpdateLaneFromFiberToRoot(fiber, update, lane);
	  }
	}
	function enqueueUpdate$1(fiber, queue, update, lane) {
	  concurrentQueues[concurrentQueuesIndex++] = fiber;
	  concurrentQueues[concurrentQueuesIndex++] = queue;
	  concurrentQueues[concurrentQueuesIndex++] = update;
	  concurrentQueues[concurrentQueuesIndex++] = lane;
	  concurrentlyUpdatedLanes |= lane;
	  fiber.lanes |= lane;
	  fiber = fiber.alternate;
	  null !== fiber && (fiber.lanes |= lane);
	}
	function enqueueConcurrentHookUpdate(fiber, queue, update, lane) {
	  enqueueUpdate$1(fiber, queue, update, lane);
	  return getRootForUpdatedFiber(fiber);
	}
	function enqueueConcurrentRenderForLane(fiber, lane) {
	  enqueueUpdate$1(fiber, null, null, lane);
	  return getRootForUpdatedFiber(fiber);
	}
	function markUpdateLaneFromFiberToRoot(sourceFiber, update, lane) {
	  sourceFiber.lanes |= lane;
	  var alternate = sourceFiber.alternate;
	  null !== alternate && (alternate.lanes |= lane);
	  for (var isHidden = !1, parent = sourceFiber.return; null !== parent; )
	    (parent.childLanes |= lane),
	      (alternate = parent.alternate),
	      null !== alternate && (alternate.childLanes |= lane),
	      22 === parent.tag &&
	        ((sourceFiber = parent.stateNode),
	        null === sourceFiber || sourceFiber._visibility & 1 || (isHidden = !0)),
	      (sourceFiber = parent),
	      (parent = parent.return);
	  return 3 === sourceFiber.tag
	    ? ((parent = sourceFiber.stateNode),
	      isHidden &&
	        null !== update &&
	        ((isHidden = 31 - clz32(lane)),
	        (sourceFiber = parent.hiddenUpdates),
	        (alternate = sourceFiber[isHidden]),
	        null === alternate
	          ? (sourceFiber[isHidden] = [update])
	          : alternate.push(update),
	        (update.lane = lane | 536870912)),
	      parent)
	    : null;
	}
	function getRootForUpdatedFiber(sourceFiber) {
	  if (50 < nestedUpdateCount)
	    throw (
	      ((nestedUpdateCount = 0),
	      (rootWithNestedUpdates = null),
	      Error(formatProdErrorMessage(185)))
	    );
	  for (var parent = sourceFiber.return; null !== parent; )
	    (sourceFiber = parent), (parent = sourceFiber.return);
	  return 3 === sourceFiber.tag ? sourceFiber.stateNode : null;
	}
	var emptyContextObject = {};
	function FiberNode(tag, pendingProps, key, mode) {
	  this.tag = tag;
	  this.key = key;
	  this.sibling =
	    this.child =
	    this.return =
	    this.stateNode =
	    this.type =
	    this.elementType =
	      null;
	  this.index = 0;
	  this.refCleanup = this.ref = null;
	  this.pendingProps = pendingProps;
	  this.dependencies =
	    this.memoizedState =
	    this.updateQueue =
	    this.memoizedProps =
	      null;
	  this.mode = mode;
	  this.subtreeFlags = this.flags = 0;
	  this.deletions = null;
	  this.childLanes = this.lanes = 0;
	  this.alternate = null;
	}
	function createFiberImplClass(tag, pendingProps, key, mode) {
	  return new FiberNode(tag, pendingProps, key, mode);
	}
	function shouldConstruct(Component) {
	  Component = Component.prototype;
	  return !(!Component || !Component.isReactComponent);
	}
	function createWorkInProgress(current, pendingProps) {
	  var workInProgress = current.alternate;
	  null === workInProgress
	    ? ((workInProgress = createFiberImplClass(
	        current.tag,
	        pendingProps,
	        current.key,
	        current.mode
	      )),
	      (workInProgress.elementType = current.elementType),
	      (workInProgress.type = current.type),
	      (workInProgress.stateNode = current.stateNode),
	      (workInProgress.alternate = current),
	      (current.alternate = workInProgress))
	    : ((workInProgress.pendingProps = pendingProps),
	      (workInProgress.type = current.type),
	      (workInProgress.flags = 0),
	      (workInProgress.subtreeFlags = 0),
	      (workInProgress.deletions = null));
	  workInProgress.flags = current.flags & 65011712;
	  workInProgress.childLanes = current.childLanes;
	  workInProgress.lanes = current.lanes;
	  workInProgress.child = current.child;
	  workInProgress.memoizedProps = current.memoizedProps;
	  workInProgress.memoizedState = current.memoizedState;
	  workInProgress.updateQueue = current.updateQueue;
	  pendingProps = current.dependencies;
	  workInProgress.dependencies =
	    null === pendingProps
	      ? null
	      : { lanes: pendingProps.lanes, firstContext: pendingProps.firstContext };
	  workInProgress.sibling = current.sibling;
	  workInProgress.index = current.index;
	  workInProgress.ref = current.ref;
	  workInProgress.refCleanup = current.refCleanup;
	  return workInProgress;
	}
	function resetWorkInProgress(workInProgress, renderLanes) {
	  workInProgress.flags &= 65011714;
	  var current = workInProgress.alternate;
	  null === current
	    ? ((workInProgress.childLanes = 0),
	      (workInProgress.lanes = renderLanes),
	      (workInProgress.child = null),
	      (workInProgress.subtreeFlags = 0),
	      (workInProgress.memoizedProps = null),
	      (workInProgress.memoizedState = null),
	      (workInProgress.updateQueue = null),
	      (workInProgress.dependencies = null),
	      (workInProgress.stateNode = null))
	    : ((workInProgress.childLanes = current.childLanes),
	      (workInProgress.lanes = current.lanes),
	      (workInProgress.child = current.child),
	      (workInProgress.subtreeFlags = 0),
	      (workInProgress.deletions = null),
	      (workInProgress.memoizedProps = current.memoizedProps),
	      (workInProgress.memoizedState = current.memoizedState),
	      (workInProgress.updateQueue = current.updateQueue),
	      (workInProgress.type = current.type),
	      (renderLanes = current.dependencies),
	      (workInProgress.dependencies =
	        null === renderLanes
	          ? null
	          : {
	              lanes: renderLanes.lanes,
	              firstContext: renderLanes.firstContext
	            }));
	  return workInProgress;
	}
	function createFiberFromTypeAndProps(
	  type,
	  key,
	  pendingProps,
	  owner,
	  mode,
	  lanes
	) {
	  var fiberTag = 0;
	  owner = type;
	  if ("function" === typeof type) shouldConstruct(type) && (fiberTag = 1);
	  else if ("string" === typeof type)
	    fiberTag = isHostHoistableType(
	      type,
	      pendingProps,
	      contextStackCursor.current
	    )
	      ? 26
	      : "html" === type || "head" === type || "body" === type
	        ? 27
	        : 5;
	  else
	    a: switch (type) {
	      case REACT_ACTIVITY_TYPE:
	        return (
	          (type = createFiberImplClass(31, pendingProps, key, mode)),
	          (type.elementType = REACT_ACTIVITY_TYPE),
	          (type.lanes = lanes),
	          type
	        );
	      case REACT_FRAGMENT_TYPE:
	        return createFiberFromFragment(pendingProps.children, mode, lanes, key);
	      case REACT_STRICT_MODE_TYPE:
	        fiberTag = 8;
	        mode |= 24;
	        break;
	      case REACT_PROFILER_TYPE:
	        return (
	          (type = createFiberImplClass(12, pendingProps, key, mode | 2)),
	          (type.elementType = REACT_PROFILER_TYPE),
	          (type.lanes = lanes),
	          type
	        );
	      case REACT_SUSPENSE_TYPE:
	        return (
	          (type = createFiberImplClass(13, pendingProps, key, mode)),
	          (type.elementType = REACT_SUSPENSE_TYPE),
	          (type.lanes = lanes),
	          type
	        );
	      case REACT_SUSPENSE_LIST_TYPE:
	        return (
	          (type = createFiberImplClass(19, pendingProps, key, mode)),
	          (type.elementType = REACT_SUSPENSE_LIST_TYPE),
	          (type.lanes = lanes),
	          type
	        );
	      default:
	        if ("object" === typeof type && null !== type)
	          switch (type.$$typeof) {
	            case REACT_CONTEXT_TYPE:
	              fiberTag = 10;
	              break a;
	            case REACT_CONSUMER_TYPE:
	              fiberTag = 9;
	              break a;
	            case REACT_FORWARD_REF_TYPE:
	              fiberTag = 11;
	              break a;
	            case REACT_MEMO_TYPE:
	              fiberTag = 14;
	              break a;
	            case REACT_LAZY_TYPE:
	              fiberTag = 16;
	              owner = null;
	              break a;
	          }
	        fiberTag = 29;
	        pendingProps = Error(
	          formatProdErrorMessage(130, null === type ? "null" : typeof type, "")
	        );
	        owner = null;
	    }
	  key = createFiberImplClass(fiberTag, pendingProps, key, mode);
	  key.elementType = type;
	  key.type = owner;
	  key.lanes = lanes;
	  return key;
	}
	function createFiberFromFragment(elements, mode, lanes, key) {
	  elements = createFiberImplClass(7, elements, key, mode);
	  elements.lanes = lanes;
	  return elements;
	}
	function createFiberFromText(content, mode, lanes) {
	  content = createFiberImplClass(6, content, null, mode);
	  content.lanes = lanes;
	  return content;
	}
	function createFiberFromDehydratedFragment(dehydratedNode) {
	  var fiber = createFiberImplClass(18, null, null, 0);
	  fiber.stateNode = dehydratedNode;
	  return fiber;
	}
	function createFiberFromPortal(portal, mode, lanes) {
	  mode = createFiberImplClass(
	    4,
	    null !== portal.children ? portal.children : [],
	    portal.key,
	    mode
	  );
	  mode.lanes = lanes;
	  mode.stateNode = {
	    containerInfo: portal.containerInfo,
	    pendingChildren: null,
	    implementation: portal.implementation
	  };
	  return mode;
	}
	var CapturedStacks = new WeakMap();
	function createCapturedValueAtFiber(value, source) {
	  if ("object" === typeof value && null !== value) {
	    var existing = CapturedStacks.get(value);
	    if (void 0 !== existing) return existing;
	    source = {
	      value: value,
	      source: source,
	      stack: getStackByFiberInDevAndProd(source)
	    };
	    CapturedStacks.set(value, source);
	    return source;
	  }
	  return {
	    value: value,
	    source: source,
	    stack: getStackByFiberInDevAndProd(source)
	  };
	}
	var forkStack = [],
	  forkStackIndex = 0,
	  treeForkProvider = null,
	  treeForkCount = 0,
	  idStack = [],
	  idStackIndex = 0,
	  treeContextProvider = null,
	  treeContextId = 1,
	  treeContextOverflow = "";
	function pushTreeFork(workInProgress, totalChildren) {
	  forkStack[forkStackIndex++] = treeForkCount;
	  forkStack[forkStackIndex++] = treeForkProvider;
	  treeForkProvider = workInProgress;
	  treeForkCount = totalChildren;
	}
	function pushTreeId(workInProgress, totalChildren, index) {
	  idStack[idStackIndex++] = treeContextId;
	  idStack[idStackIndex++] = treeContextOverflow;
	  idStack[idStackIndex++] = treeContextProvider;
	  treeContextProvider = workInProgress;
	  var baseIdWithLeadingBit = treeContextId;
	  workInProgress = treeContextOverflow;
	  var baseLength = 32 - clz32(baseIdWithLeadingBit) - 1;
	  baseIdWithLeadingBit &= ~(1 << baseLength);
	  index += 1;
	  var length = 32 - clz32(totalChildren) + baseLength;
	  if (30 < length) {
	    var numberOfOverflowBits = baseLength - (baseLength % 5);
	    length = (
	      baseIdWithLeadingBit &
	      ((1 << numberOfOverflowBits) - 1)
	    ).toString(32);
	    baseIdWithLeadingBit >>= numberOfOverflowBits;
	    baseLength -= numberOfOverflowBits;
	    treeContextId =
	      (1 << (32 - clz32(totalChildren) + baseLength)) |
	      (index << baseLength) |
	      baseIdWithLeadingBit;
	    treeContextOverflow = length + workInProgress;
	  } else
	    (treeContextId =
	      (1 << length) | (index << baseLength) | baseIdWithLeadingBit),
	      (treeContextOverflow = workInProgress);
	}
	function pushMaterializedTreeId(workInProgress) {
	  null !== workInProgress.return &&
	    (pushTreeFork(workInProgress, 1), pushTreeId(workInProgress, 1, 0));
	}
	function popTreeContext(workInProgress) {
	  for (; workInProgress === treeForkProvider; )
	    (treeForkProvider = forkStack[--forkStackIndex]),
	      (forkStack[forkStackIndex] = null),
	      (treeForkCount = forkStack[--forkStackIndex]),
	      (forkStack[forkStackIndex] = null);
	  for (; workInProgress === treeContextProvider; )
	    (treeContextProvider = idStack[--idStackIndex]),
	      (idStack[idStackIndex] = null),
	      (treeContextOverflow = idStack[--idStackIndex]),
	      (idStack[idStackIndex] = null),
	      (treeContextId = idStack[--idStackIndex]),
	      (idStack[idStackIndex] = null);
	}
	function restoreSuspendedTreeContext(workInProgress, suspendedContext) {
	  idStack[idStackIndex++] = treeContextId;
	  idStack[idStackIndex++] = treeContextOverflow;
	  idStack[idStackIndex++] = treeContextProvider;
	  treeContextId = suspendedContext.id;
	  treeContextOverflow = suspendedContext.overflow;
	  treeContextProvider = workInProgress;
	}
	var hydrationParentFiber = null,
	  nextHydratableInstance = null,
	  isHydrating = !1,
	  hydrationErrors = null,
	  rootOrSingletonContext = !1,
	  HydrationMismatchException = Error(formatProdErrorMessage(519));
	function throwOnHydrationMismatch(fiber) {
	  var error = Error(
	    formatProdErrorMessage(
	      418,
	      1 < arguments.length && void 0 !== arguments[1] && arguments[1]
	        ? "text"
	        : "HTML",
	      ""
	    )
	  );
	  queueHydrationError(createCapturedValueAtFiber(error, fiber));
	  throw HydrationMismatchException;
	}
	function prepareToHydrateHostInstance(fiber) {
	  var instance = fiber.stateNode,
	    type = fiber.type,
	    props = fiber.memoizedProps;
	  instance[internalInstanceKey] = fiber;
	  instance[internalPropsKey] = props;
	  switch (type) {
	    case "dialog":
	      listenToNonDelegatedEvent("cancel", instance);
	      listenToNonDelegatedEvent("close", instance);
	      break;
	    case "iframe":
	    case "object":
	    case "embed":
	      listenToNonDelegatedEvent("load", instance);
	      break;
	    case "video":
	    case "audio":
	      for (type = 0; type < mediaEventTypes.length; type++)
	        listenToNonDelegatedEvent(mediaEventTypes[type], instance);
	      break;
	    case "source":
	      listenToNonDelegatedEvent("error", instance);
	      break;
	    case "img":
	    case "image":
	    case "link":
	      listenToNonDelegatedEvent("error", instance);
	      listenToNonDelegatedEvent("load", instance);
	      break;
	    case "details":
	      listenToNonDelegatedEvent("toggle", instance);
	      break;
	    case "input":
	      listenToNonDelegatedEvent("invalid", instance);
	      initInput(
	        instance,
	        props.value,
	        props.defaultValue,
	        props.checked,
	        props.defaultChecked,
	        props.type,
	        props.name,
	        !0
	      );
	      break;
	    case "select":
	      listenToNonDelegatedEvent("invalid", instance);
	      break;
	    case "textarea":
	      listenToNonDelegatedEvent("invalid", instance),
	        initTextarea(instance, props.value, props.defaultValue, props.children);
	  }
	  type = props.children;
	  ("string" !== typeof type &&
	    "number" !== typeof type &&
	    "bigint" !== typeof type) ||
	  instance.textContent === "" + type ||
	  !0 === props.suppressHydrationWarning ||
	  checkForUnmatchedText(instance.textContent, type)
	    ? (null != props.popover &&
	        (listenToNonDelegatedEvent("beforetoggle", instance),
	        listenToNonDelegatedEvent("toggle", instance)),
	      null != props.onScroll && listenToNonDelegatedEvent("scroll", instance),
	      null != props.onScrollEnd &&
	        listenToNonDelegatedEvent("scrollend", instance),
	      null != props.onClick && (instance.onclick = noop$1),
	      (instance = !0))
	    : (instance = !1);
	  instance || throwOnHydrationMismatch(fiber, !0);
	}
	function popToNextHostParent(fiber) {
	  for (hydrationParentFiber = fiber.return; hydrationParentFiber; )
	    switch (hydrationParentFiber.tag) {
	      case 5:
	      case 31:
	      case 13:
	        rootOrSingletonContext = !1;
	        return;
	      case 27:
	      case 3:
	        rootOrSingletonContext = !0;
	        return;
	      default:
	        hydrationParentFiber = hydrationParentFiber.return;
	    }
	}
	function popHydrationState(fiber) {
	  if (fiber !== hydrationParentFiber) return !1;
	  if (!isHydrating) return popToNextHostParent(fiber), (isHydrating = !0), !1;
	  var tag = fiber.tag,
	    JSCompiler_temp;
	  if ((JSCompiler_temp = 3 !== tag && 27 !== tag)) {
	    if ((JSCompiler_temp = 5 === tag))
	      (JSCompiler_temp = fiber.type),
	        (JSCompiler_temp =
	          !("form" !== JSCompiler_temp && "button" !== JSCompiler_temp) ||
	          shouldSetTextContent(fiber.type, fiber.memoizedProps));
	    JSCompiler_temp = !JSCompiler_temp;
	  }
	  JSCompiler_temp && nextHydratableInstance && throwOnHydrationMismatch(fiber);
	  popToNextHostParent(fiber);
	  if (13 === tag) {
	    fiber = fiber.memoizedState;
	    fiber = null !== fiber ? fiber.dehydrated : null;
	    if (!fiber) throw Error(formatProdErrorMessage(317));
	    nextHydratableInstance =
	      getNextHydratableInstanceAfterHydrationBoundary(fiber);
	  } else if (31 === tag) {
	    fiber = fiber.memoizedState;
	    fiber = null !== fiber ? fiber.dehydrated : null;
	    if (!fiber) throw Error(formatProdErrorMessage(317));
	    nextHydratableInstance =
	      getNextHydratableInstanceAfterHydrationBoundary(fiber);
	  } else
	    27 === tag
	      ? ((tag = nextHydratableInstance),
	        isSingletonScope(fiber.type)
	          ? ((fiber = previousHydratableOnEnteringScopedSingleton),
	            (previousHydratableOnEnteringScopedSingleton = null),
	            (nextHydratableInstance = fiber))
	          : (nextHydratableInstance = tag))
	      : (nextHydratableInstance = hydrationParentFiber
	          ? getNextHydratable(fiber.stateNode.nextSibling)
	          : null);
	  return !0;
	}
	function resetHydrationState() {
	  nextHydratableInstance = hydrationParentFiber = null;
	  isHydrating = !1;
	}
	function upgradeHydrationErrorsToRecoverable() {
	  var queuedErrors = hydrationErrors;
	  null !== queuedErrors &&
	    (null === workInProgressRootRecoverableErrors
	      ? (workInProgressRootRecoverableErrors = queuedErrors)
	      : workInProgressRootRecoverableErrors.push.apply(
	          workInProgressRootRecoverableErrors,
	          queuedErrors
	        ),
	    (hydrationErrors = null));
	  return queuedErrors;
	}
	function queueHydrationError(error) {
	  null === hydrationErrors
	    ? (hydrationErrors = [error])
	    : hydrationErrors.push(error);
	}
	var valueCursor = createCursor(null),
	  currentlyRenderingFiber$1 = null,
	  lastContextDependency = null;
	function pushProvider(providerFiber, context, nextValue) {
	  push(valueCursor, context._currentValue);
	  context._currentValue = nextValue;
	}
	function popProvider(context) {
	  context._currentValue = valueCursor.current;
	  pop(valueCursor);
	}
	function scheduleContextWorkOnParentPath(parent, renderLanes, propagationRoot) {
	  for (; null !== parent; ) {
	    var alternate = parent.alternate;
	    (parent.childLanes & renderLanes) !== renderLanes
	      ? ((parent.childLanes |= renderLanes),
	        null !== alternate && (alternate.childLanes |= renderLanes))
	      : null !== alternate &&
	        (alternate.childLanes & renderLanes) !== renderLanes &&
	        (alternate.childLanes |= renderLanes);
	    if (parent === propagationRoot) break;
	    parent = parent.return;
	  }
	}
	function propagateContextChanges(
	  workInProgress,
	  contexts,
	  renderLanes,
	  forcePropagateEntireTree
	) {
	  var fiber = workInProgress.child;
	  null !== fiber && (fiber.return = workInProgress);
	  for (; null !== fiber; ) {
	    var list = fiber.dependencies;
	    if (null !== list) {
	      var nextFiber = fiber.child;
	      list = list.firstContext;
	      a: for (; null !== list; ) {
	        var dependency = list;
	        list = fiber;
	        for (var i = 0; i < contexts.length; i++)
	          if (dependency.context === contexts[i]) {
	            list.lanes |= renderLanes;
	            dependency = list.alternate;
	            null !== dependency && (dependency.lanes |= renderLanes);
	            scheduleContextWorkOnParentPath(
	              list.return,
	              renderLanes,
	              workInProgress
	            );
	            forcePropagateEntireTree || (nextFiber = null);
	            break a;
	          }
	        list = dependency.next;
	      }
	    } else if (18 === fiber.tag) {
	      nextFiber = fiber.return;
	      if (null === nextFiber) throw Error(formatProdErrorMessage(341));
	      nextFiber.lanes |= renderLanes;
	      list = nextFiber.alternate;
	      null !== list && (list.lanes |= renderLanes);
	      scheduleContextWorkOnParentPath(nextFiber, renderLanes, workInProgress);
	      nextFiber = null;
	    } else nextFiber = fiber.child;
	    if (null !== nextFiber) nextFiber.return = fiber;
	    else
	      for (nextFiber = fiber; null !== nextFiber; ) {
	        if (nextFiber === workInProgress) {
	          nextFiber = null;
	          break;
	        }
	        fiber = nextFiber.sibling;
	        if (null !== fiber) {
	          fiber.return = nextFiber.return;
	          nextFiber = fiber;
	          break;
	        }
	        nextFiber = nextFiber.return;
	      }
	    fiber = nextFiber;
	  }
	}
	function propagateParentContextChanges(
	  current,
	  workInProgress,
	  renderLanes,
	  forcePropagateEntireTree
	) {
	  current = null;
	  for (
	    var parent = workInProgress, isInsidePropagationBailout = !1;
	    null !== parent;

	  ) {
	    if (!isInsidePropagationBailout)
	      if (0 !== (parent.flags & 524288)) isInsidePropagationBailout = !0;
	      else if (0 !== (parent.flags & 262144)) break;
	    if (10 === parent.tag) {
	      var currentParent = parent.alternate;
	      if (null === currentParent) throw Error(formatProdErrorMessage(387));
	      currentParent = currentParent.memoizedProps;
	      if (null !== currentParent) {
	        var context = parent.type;
	        objectIs(parent.pendingProps.value, currentParent.value) ||
	          (null !== current ? current.push(context) : (current = [context]));
	      }
	    } else if (parent === hostTransitionProviderCursor.current) {
	      currentParent = parent.alternate;
	      if (null === currentParent) throw Error(formatProdErrorMessage(387));
	      currentParent.memoizedState.memoizedState !==
	        parent.memoizedState.memoizedState &&
	        (null !== current
	          ? current.push(HostTransitionContext)
	          : (current = [HostTransitionContext]));
	    }
	    parent = parent.return;
	  }
	  null !== current &&
	    propagateContextChanges(
	      workInProgress,
	      current,
	      renderLanes,
	      forcePropagateEntireTree
	    );
	  workInProgress.flags |= 262144;
	}
	function checkIfContextChanged(currentDependencies) {
	  for (
	    currentDependencies = currentDependencies.firstContext;
	    null !== currentDependencies;

	  ) {
	    if (
	      !objectIs(
	        currentDependencies.context._currentValue,
	        currentDependencies.memoizedValue
	      )
	    )
	      return !0;
	    currentDependencies = currentDependencies.next;
	  }
	  return !1;
	}
	function prepareToReadContext(workInProgress) {
	  currentlyRenderingFiber$1 = workInProgress;
	  lastContextDependency = null;
	  workInProgress = workInProgress.dependencies;
	  null !== workInProgress && (workInProgress.firstContext = null);
	}
	function readContext(context) {
	  return readContextForConsumer(currentlyRenderingFiber$1, context);
	}
	function readContextDuringReconciliation(consumer, context) {
	  null === currentlyRenderingFiber$1 && prepareToReadContext(consumer);
	  return readContextForConsumer(consumer, context);
	}
	function readContextForConsumer(consumer, context) {
	  var value = context._currentValue;
	  context = { context: context, memoizedValue: value, next: null };
	  if (null === lastContextDependency) {
	    if (null === consumer) throw Error(formatProdErrorMessage(308));
	    lastContextDependency = context;
	    consumer.dependencies = { lanes: 0, firstContext: context };
	    consumer.flags |= 524288;
	  } else lastContextDependency = lastContextDependency.next = context;
	  return value;
	}
	var AbortControllerLocal =
	    "undefined" !== typeof AbortController
	      ? AbortController
	      : function () {
	          var listeners = [],
	            signal = (this.signal = {
	              aborted: !1,
	              addEventListener: function (type, listener) {
	                listeners.push(listener);
	              }
	            });
	          this.abort = function () {
	            signal.aborted = !0;
	            listeners.forEach(function (listener) {
	              return listener();
	            });
	          };
	        },
	  scheduleCallback$2 = Scheduler.unstable_scheduleCallback,
	  NormalPriority = Scheduler.unstable_NormalPriority,
	  CacheContext = {
	    $$typeof: REACT_CONTEXT_TYPE,
	    Consumer: null,
	    Provider: null,
	    _currentValue: null,
	    _currentValue2: null,
	    _threadCount: 0
	  };
	function createCache() {
	  return {
	    controller: new AbortControllerLocal(),
	    data: new Map(),
	    refCount: 0
	  };
	}
	function releaseCache(cache) {
	  cache.refCount--;
	  0 === cache.refCount &&
	    scheduleCallback$2(NormalPriority, function () {
	      cache.controller.abort();
	    });
	}
	var currentEntangledListeners = null,
	  currentEntangledPendingCount = 0,
	  currentEntangledLane = 0,
	  currentEntangledActionThenable = null;
	function entangleAsyncAction(transition, thenable) {
	  if (null === currentEntangledListeners) {
	    var entangledListeners = (currentEntangledListeners = []);
	    currentEntangledPendingCount = 0;
	    currentEntangledLane = requestTransitionLane();
	    currentEntangledActionThenable = {
	      status: "pending",
	      value: void 0,
	      then: function (resolve) {
	        entangledListeners.push(resolve);
	      }
	    };
	  }
	  currentEntangledPendingCount++;
	  thenable.then(pingEngtangledActionScope, pingEngtangledActionScope);
	  return thenable;
	}
	function pingEngtangledActionScope() {
	  if (
	    0 === --currentEntangledPendingCount &&
	    null !== currentEntangledListeners
	  ) {
	    null !== currentEntangledActionThenable &&
	      (currentEntangledActionThenable.status = "fulfilled");
	    var listeners = currentEntangledListeners;
	    currentEntangledListeners = null;
	    currentEntangledLane = 0;
	    currentEntangledActionThenable = null;
	    for (var i = 0; i < listeners.length; i++) (0, listeners[i])();
	  }
	}
	function chainThenableValue(thenable, result) {
	  var listeners = [],
	    thenableWithOverride = {
	      status: "pending",
	      value: null,
	      reason: null,
	      then: function (resolve) {
	        listeners.push(resolve);
	      }
	    };
	  thenable.then(
	    function () {
	      thenableWithOverride.status = "fulfilled";
	      thenableWithOverride.value = result;
	      for (var i = 0; i < listeners.length; i++) (0, listeners[i])(result);
	    },
	    function (error) {
	      thenableWithOverride.status = "rejected";
	      thenableWithOverride.reason = error;
	      for (error = 0; error < listeners.length; error++)
	        (0, listeners[error])(void 0);
	    }
	  );
	  return thenableWithOverride;
	}
	var prevOnStartTransitionFinish = ReactSharedInternals.S;
	ReactSharedInternals.S = function (transition, returnValue) {
	  globalMostRecentTransitionTime = now();
	  "object" === typeof returnValue &&
	    null !== returnValue &&
	    "function" === typeof returnValue.then &&
	    entangleAsyncAction(transition, returnValue);
	  null !== prevOnStartTransitionFinish &&
	    prevOnStartTransitionFinish(transition, returnValue);
	};
	var resumedCache = createCursor(null);
	function peekCacheFromPool() {
	  var cacheResumedFromPreviousRender = resumedCache.current;
	  return null !== cacheResumedFromPreviousRender
	    ? cacheResumedFromPreviousRender
	    : workInProgressRoot.pooledCache;
	}
	function pushTransition(offscreenWorkInProgress, prevCachePool) {
	  null === prevCachePool
	    ? push(resumedCache, resumedCache.current)
	    : push(resumedCache, prevCachePool.pool);
	}
	function getSuspendedCache() {
	  var cacheFromPool = peekCacheFromPool();
	  return null === cacheFromPool
	    ? null
	    : { parent: CacheContext._currentValue, pool: cacheFromPool };
	}
	var SuspenseException = Error(formatProdErrorMessage(460)),
	  SuspenseyCommitException = Error(formatProdErrorMessage(474)),
	  SuspenseActionException = Error(formatProdErrorMessage(542)),
	  noopSuspenseyCommitThenable = { then: function () {} };
	function isThenableResolved(thenable) {
	  thenable = thenable.status;
	  return "fulfilled" === thenable || "rejected" === thenable;
	}
	function trackUsedThenable(thenableState, thenable, index) {
	  index = thenableState[index];
	  void 0 === index
	    ? thenableState.push(thenable)
	    : index !== thenable && (thenable.then(noop$1, noop$1), (thenable = index));
	  switch (thenable.status) {
	    case "fulfilled":
	      return thenable.value;
	    case "rejected":
	      throw (
	        ((thenableState = thenable.reason),
	        checkIfUseWrappedInAsyncCatch(thenableState),
	        thenableState)
	      );
	    default:
	      if ("string" === typeof thenable.status) thenable.then(noop$1, noop$1);
	      else {
	        thenableState = workInProgressRoot;
	        if (null !== thenableState && 100 < thenableState.shellSuspendCounter)
	          throw Error(formatProdErrorMessage(482));
	        thenableState = thenable;
	        thenableState.status = "pending";
	        thenableState.then(
	          function (fulfilledValue) {
	            if ("pending" === thenable.status) {
	              var fulfilledThenable = thenable;
	              fulfilledThenable.status = "fulfilled";
	              fulfilledThenable.value = fulfilledValue;
	            }
	          },
	          function (error) {
	            if ("pending" === thenable.status) {
	              var rejectedThenable = thenable;
	              rejectedThenable.status = "rejected";
	              rejectedThenable.reason = error;
	            }
	          }
	        );
	      }
	      switch (thenable.status) {
	        case "fulfilled":
	          return thenable.value;
	        case "rejected":
	          throw (
	            ((thenableState = thenable.reason),
	            checkIfUseWrappedInAsyncCatch(thenableState),
	            thenableState)
	          );
	      }
	      suspendedThenable = thenable;
	      throw SuspenseException;
	  }
	}
	function resolveLazy(lazyType) {
	  try {
	    var init = lazyType._init;
	    return init(lazyType._payload);
	  } catch (x) {
	    if (null !== x && "object" === typeof x && "function" === typeof x.then)
	      throw ((suspendedThenable = x), SuspenseException);
	    throw x;
	  }
	}
	var suspendedThenable = null;
	function getSuspendedThenable() {
	  if (null === suspendedThenable) throw Error(formatProdErrorMessage(459));
	  var thenable = suspendedThenable;
	  suspendedThenable = null;
	  return thenable;
	}
	function checkIfUseWrappedInAsyncCatch(rejectedReason) {
	  if (
	    rejectedReason === SuspenseException ||
	    rejectedReason === SuspenseActionException
	  )
	    throw Error(formatProdErrorMessage(483));
	}
	var thenableState$1 = null,
	  thenableIndexCounter$1 = 0;
	function unwrapThenable(thenable) {
	  var index = thenableIndexCounter$1;
	  thenableIndexCounter$1 += 1;
	  null === thenableState$1 && (thenableState$1 = []);
	  return trackUsedThenable(thenableState$1, thenable, index);
	}
	function coerceRef(workInProgress, element) {
	  element = element.props.ref;
	  workInProgress.ref = void 0 !== element ? element : null;
	}
	function throwOnInvalidObjectTypeImpl(returnFiber, newChild) {
	  if (newChild.$$typeof === REACT_LEGACY_ELEMENT_TYPE)
	    throw Error(formatProdErrorMessage(525));
	  returnFiber = Object.prototype.toString.call(newChild);
	  throw Error(
	    formatProdErrorMessage(
	      31,
	      "[object Object]" === returnFiber
	        ? "object with keys {" + Object.keys(newChild).join(", ") + "}"
	        : returnFiber
	    )
	  );
	}
	function createChildReconciler(shouldTrackSideEffects) {
	  function deleteChild(returnFiber, childToDelete) {
	    if (shouldTrackSideEffects) {
	      var deletions = returnFiber.deletions;
	      null === deletions
	        ? ((returnFiber.deletions = [childToDelete]), (returnFiber.flags |= 16))
	        : deletions.push(childToDelete);
	    }
	  }
	  function deleteRemainingChildren(returnFiber, currentFirstChild) {
	    if (!shouldTrackSideEffects) return null;
	    for (; null !== currentFirstChild; )
	      deleteChild(returnFiber, currentFirstChild),
	        (currentFirstChild = currentFirstChild.sibling);
	    return null;
	  }
	  function mapRemainingChildren(currentFirstChild) {
	    for (var existingChildren = new Map(); null !== currentFirstChild; )
	      null !== currentFirstChild.key
	        ? existingChildren.set(currentFirstChild.key, currentFirstChild)
	        : existingChildren.set(currentFirstChild.index, currentFirstChild),
	        (currentFirstChild = currentFirstChild.sibling);
	    return existingChildren;
	  }
	  function useFiber(fiber, pendingProps) {
	    fiber = createWorkInProgress(fiber, pendingProps);
	    fiber.index = 0;
	    fiber.sibling = null;
	    return fiber;
	  }
	  function placeChild(newFiber, lastPlacedIndex, newIndex) {
	    newFiber.index = newIndex;
	    if (!shouldTrackSideEffects)
	      return (newFiber.flags |= 1048576), lastPlacedIndex;
	    newIndex = newFiber.alternate;
	    if (null !== newIndex)
	      return (
	        (newIndex = newIndex.index),
	        newIndex < lastPlacedIndex
	          ? ((newFiber.flags |= 67108866), lastPlacedIndex)
	          : newIndex
	      );
	    newFiber.flags |= 67108866;
	    return lastPlacedIndex;
	  }
	  function placeSingleChild(newFiber) {
	    shouldTrackSideEffects &&
	      null === newFiber.alternate &&
	      (newFiber.flags |= 67108866);
	    return newFiber;
	  }
	  function updateTextNode(returnFiber, current, textContent, lanes) {
	    if (null === current || 6 !== current.tag)
	      return (
	        (current = createFiberFromText(textContent, returnFiber.mode, lanes)),
	        (current.return = returnFiber),
	        current
	      );
	    current = useFiber(current, textContent);
	    current.return = returnFiber;
	    return current;
	  }
	  function updateElement(returnFiber, current, element, lanes) {
	    var elementType = element.type;
	    if (elementType === REACT_FRAGMENT_TYPE)
	      return updateFragment(
	        returnFiber,
	        current,
	        element.props.children,
	        lanes,
	        element.key
	      );
	    if (
	      null !== current &&
	      (current.elementType === elementType ||
	        ("object" === typeof elementType &&
	          null !== elementType &&
	          elementType.$$typeof === REACT_LAZY_TYPE &&
	          resolveLazy(elementType) === current.type))
	    )
	      return (
	        (current = useFiber(current, element.props)),
	        coerceRef(current, element),
	        (current.return = returnFiber),
	        current
	      );
	    current = createFiberFromTypeAndProps(
	      element.type,
	      element.key,
	      element.props,
	      null,
	      returnFiber.mode,
	      lanes
	    );
	    coerceRef(current, element);
	    current.return = returnFiber;
	    return current;
	  }
	  function updatePortal(returnFiber, current, portal, lanes) {
	    if (
	      null === current ||
	      4 !== current.tag ||
	      current.stateNode.containerInfo !== portal.containerInfo ||
	      current.stateNode.implementation !== portal.implementation
	    )
	      return (
	        (current = createFiberFromPortal(portal, returnFiber.mode, lanes)),
	        (current.return = returnFiber),
	        current
	      );
	    current = useFiber(current, portal.children || []);
	    current.return = returnFiber;
	    return current;
	  }
	  function updateFragment(returnFiber, current, fragment, lanes, key) {
	    if (null === current || 7 !== current.tag)
	      return (
	        (current = createFiberFromFragment(
	          fragment,
	          returnFiber.mode,
	          lanes,
	          key
	        )),
	        (current.return = returnFiber),
	        current
	      );
	    current = useFiber(current, fragment);
	    current.return = returnFiber;
	    return current;
	  }
	  function createChild(returnFiber, newChild, lanes) {
	    if (
	      ("string" === typeof newChild && "" !== newChild) ||
	      "number" === typeof newChild ||
	      "bigint" === typeof newChild
	    )
	      return (
	        (newChild = createFiberFromText(
	          "" + newChild,
	          returnFiber.mode,
	          lanes
	        )),
	        (newChild.return = returnFiber),
	        newChild
	      );
	    if ("object" === typeof newChild && null !== newChild) {
	      switch (newChild.$$typeof) {
	        case REACT_ELEMENT_TYPE:
	          return (
	            (lanes = createFiberFromTypeAndProps(
	              newChild.type,
	              newChild.key,
	              newChild.props,
	              null,
	              returnFiber.mode,
	              lanes
	            )),
	            coerceRef(lanes, newChild),
	            (lanes.return = returnFiber),
	            lanes
	          );
	        case REACT_PORTAL_TYPE:
	          return (
	            (newChild = createFiberFromPortal(
	              newChild,
	              returnFiber.mode,
	              lanes
	            )),
	            (newChild.return = returnFiber),
	            newChild
	          );
	        case REACT_LAZY_TYPE:
	          return (
	            (newChild = resolveLazy(newChild)),
	            createChild(returnFiber, newChild, lanes)
	          );
	      }
	      if (isArrayImpl(newChild) || getIteratorFn(newChild))
	        return (
	          (newChild = createFiberFromFragment(
	            newChild,
	            returnFiber.mode,
	            lanes,
	            null
	          )),
	          (newChild.return = returnFiber),
	          newChild
	        );
	      if ("function" === typeof newChild.then)
	        return createChild(returnFiber, unwrapThenable(newChild), lanes);
	      if (newChild.$$typeof === REACT_CONTEXT_TYPE)
	        return createChild(
	          returnFiber,
	          readContextDuringReconciliation(returnFiber, newChild),
	          lanes
	        );
	      throwOnInvalidObjectTypeImpl(returnFiber, newChild);
	    }
	    return null;
	  }
	  function updateSlot(returnFiber, oldFiber, newChild, lanes) {
	    var key = null !== oldFiber ? oldFiber.key : null;
	    if (
	      ("string" === typeof newChild && "" !== newChild) ||
	      "number" === typeof newChild ||
	      "bigint" === typeof newChild
	    )
	      return null !== key
	        ? null
	        : updateTextNode(returnFiber, oldFiber, "" + newChild, lanes);
	    if ("object" === typeof newChild && null !== newChild) {
	      switch (newChild.$$typeof) {
	        case REACT_ELEMENT_TYPE:
	          return newChild.key === key
	            ? updateElement(returnFiber, oldFiber, newChild, lanes)
	            : null;
	        case REACT_PORTAL_TYPE:
	          return newChild.key === key
	            ? updatePortal(returnFiber, oldFiber, newChild, lanes)
	            : null;
	        case REACT_LAZY_TYPE:
	          return (
	            (newChild = resolveLazy(newChild)),
	            updateSlot(returnFiber, oldFiber, newChild, lanes)
	          );
	      }
	      if (isArrayImpl(newChild) || getIteratorFn(newChild))
	        return null !== key
	          ? null
	          : updateFragment(returnFiber, oldFiber, newChild, lanes, null);
	      if ("function" === typeof newChild.then)
	        return updateSlot(
	          returnFiber,
	          oldFiber,
	          unwrapThenable(newChild),
	          lanes
	        );
	      if (newChild.$$typeof === REACT_CONTEXT_TYPE)
	        return updateSlot(
	          returnFiber,
	          oldFiber,
	          readContextDuringReconciliation(returnFiber, newChild),
	          lanes
	        );
	      throwOnInvalidObjectTypeImpl(returnFiber, newChild);
	    }
	    return null;
	  }
	  function updateFromMap(
	    existingChildren,
	    returnFiber,
	    newIdx,
	    newChild,
	    lanes
	  ) {
	    if (
	      ("string" === typeof newChild && "" !== newChild) ||
	      "number" === typeof newChild ||
	      "bigint" === typeof newChild
	    )
	      return (
	        (existingChildren = existingChildren.get(newIdx) || null),
	        updateTextNode(returnFiber, existingChildren, "" + newChild, lanes)
	      );
	    if ("object" === typeof newChild && null !== newChild) {
	      switch (newChild.$$typeof) {
	        case REACT_ELEMENT_TYPE:
	          return (
	            (existingChildren =
	              existingChildren.get(
	                null === newChild.key ? newIdx : newChild.key
	              ) || null),
	            updateElement(returnFiber, existingChildren, newChild, lanes)
	          );
	        case REACT_PORTAL_TYPE:
	          return (
	            (existingChildren =
	              existingChildren.get(
	                null === newChild.key ? newIdx : newChild.key
	              ) || null),
	            updatePortal(returnFiber, existingChildren, newChild, lanes)
	          );
	        case REACT_LAZY_TYPE:
	          return (
	            (newChild = resolveLazy(newChild)),
	            updateFromMap(
	              existingChildren,
	              returnFiber,
	              newIdx,
	              newChild,
	              lanes
	            )
	          );
	      }
	      if (isArrayImpl(newChild) || getIteratorFn(newChild))
	        return (
	          (existingChildren = existingChildren.get(newIdx) || null),
	          updateFragment(returnFiber, existingChildren, newChild, lanes, null)
	        );
	      if ("function" === typeof newChild.then)
	        return updateFromMap(
	          existingChildren,
	          returnFiber,
	          newIdx,
	          unwrapThenable(newChild),
	          lanes
	        );
	      if (newChild.$$typeof === REACT_CONTEXT_TYPE)
	        return updateFromMap(
	          existingChildren,
	          returnFiber,
	          newIdx,
	          readContextDuringReconciliation(returnFiber, newChild),
	          lanes
	        );
	      throwOnInvalidObjectTypeImpl(returnFiber, newChild);
	    }
	    return null;
	  }
	  function reconcileChildrenArray(
	    returnFiber,
	    currentFirstChild,
	    newChildren,
	    lanes
	  ) {
	    for (
	      var resultingFirstChild = null,
	        previousNewFiber = null,
	        oldFiber = currentFirstChild,
	        newIdx = (currentFirstChild = 0),
	        nextOldFiber = null;
	      null !== oldFiber && newIdx < newChildren.length;
	      newIdx++
	    ) {
	      oldFiber.index > newIdx
	        ? ((nextOldFiber = oldFiber), (oldFiber = null))
	        : (nextOldFiber = oldFiber.sibling);
	      var newFiber = updateSlot(
	        returnFiber,
	        oldFiber,
	        newChildren[newIdx],
	        lanes
	      );
	      if (null === newFiber) {
	        null === oldFiber && (oldFiber = nextOldFiber);
	        break;
	      }
	      shouldTrackSideEffects &&
	        oldFiber &&
	        null === newFiber.alternate &&
	        deleteChild(returnFiber, oldFiber);
	      currentFirstChild = placeChild(newFiber, currentFirstChild, newIdx);
	      null === previousNewFiber
	        ? (resultingFirstChild = newFiber)
	        : (previousNewFiber.sibling = newFiber);
	      previousNewFiber = newFiber;
	      oldFiber = nextOldFiber;
	    }
	    if (newIdx === newChildren.length)
	      return (
	        deleteRemainingChildren(returnFiber, oldFiber),
	        isHydrating && pushTreeFork(returnFiber, newIdx),
	        resultingFirstChild
	      );
	    if (null === oldFiber) {
	      for (; newIdx < newChildren.length; newIdx++)
	        (oldFiber = createChild(returnFiber, newChildren[newIdx], lanes)),
	          null !== oldFiber &&
	            ((currentFirstChild = placeChild(
	              oldFiber,
	              currentFirstChild,
	              newIdx
	            )),
	            null === previousNewFiber
	              ? (resultingFirstChild = oldFiber)
	              : (previousNewFiber.sibling = oldFiber),
	            (previousNewFiber = oldFiber));
	      isHydrating && pushTreeFork(returnFiber, newIdx);
	      return resultingFirstChild;
	    }
	    for (
	      oldFiber = mapRemainingChildren(oldFiber);
	      newIdx < newChildren.length;
	      newIdx++
	    )
	      (nextOldFiber = updateFromMap(
	        oldFiber,
	        returnFiber,
	        newIdx,
	        newChildren[newIdx],
	        lanes
	      )),
	        null !== nextOldFiber &&
	          (shouldTrackSideEffects &&
	            null !== nextOldFiber.alternate &&
	            oldFiber.delete(
	              null === nextOldFiber.key ? newIdx : nextOldFiber.key
	            ),
	          (currentFirstChild = placeChild(
	            nextOldFiber,
	            currentFirstChild,
	            newIdx
	          )),
	          null === previousNewFiber
	            ? (resultingFirstChild = nextOldFiber)
	            : (previousNewFiber.sibling = nextOldFiber),
	          (previousNewFiber = nextOldFiber));
	    shouldTrackSideEffects &&
	      oldFiber.forEach(function (child) {
	        return deleteChild(returnFiber, child);
	      });
	    isHydrating && pushTreeFork(returnFiber, newIdx);
	    return resultingFirstChild;
	  }
	  function reconcileChildrenIterator(
	    returnFiber,
	    currentFirstChild,
	    newChildren,
	    lanes
	  ) {
	    if (null == newChildren) throw Error(formatProdErrorMessage(151));
	    for (
	      var resultingFirstChild = null,
	        previousNewFiber = null,
	        oldFiber = currentFirstChild,
	        newIdx = (currentFirstChild = 0),
	        nextOldFiber = null,
	        step = newChildren.next();
	      null !== oldFiber && !step.done;
	      newIdx++, step = newChildren.next()
	    ) {
	      oldFiber.index > newIdx
	        ? ((nextOldFiber = oldFiber), (oldFiber = null))
	        : (nextOldFiber = oldFiber.sibling);
	      var newFiber = updateSlot(returnFiber, oldFiber, step.value, lanes);
	      if (null === newFiber) {
	        null === oldFiber && (oldFiber = nextOldFiber);
	        break;
	      }
	      shouldTrackSideEffects &&
	        oldFiber &&
	        null === newFiber.alternate &&
	        deleteChild(returnFiber, oldFiber);
	      currentFirstChild = placeChild(newFiber, currentFirstChild, newIdx);
	      null === previousNewFiber
	        ? (resultingFirstChild = newFiber)
	        : (previousNewFiber.sibling = newFiber);
	      previousNewFiber = newFiber;
	      oldFiber = nextOldFiber;
	    }
	    if (step.done)
	      return (
	        deleteRemainingChildren(returnFiber, oldFiber),
	        isHydrating && pushTreeFork(returnFiber, newIdx),
	        resultingFirstChild
	      );
	    if (null === oldFiber) {
	      for (; !step.done; newIdx++, step = newChildren.next())
	        (step = createChild(returnFiber, step.value, lanes)),
	          null !== step &&
	            ((currentFirstChild = placeChild(step, currentFirstChild, newIdx)),
	            null === previousNewFiber
	              ? (resultingFirstChild = step)
	              : (previousNewFiber.sibling = step),
	            (previousNewFiber = step));
	      isHydrating && pushTreeFork(returnFiber, newIdx);
	      return resultingFirstChild;
	    }
	    for (
	      oldFiber = mapRemainingChildren(oldFiber);
	      !step.done;
	      newIdx++, step = newChildren.next()
	    )
	      (step = updateFromMap(oldFiber, returnFiber, newIdx, step.value, lanes)),
	        null !== step &&
	          (shouldTrackSideEffects &&
	            null !== step.alternate &&
	            oldFiber.delete(null === step.key ? newIdx : step.key),
	          (currentFirstChild = placeChild(step, currentFirstChild, newIdx)),
	          null === previousNewFiber
	            ? (resultingFirstChild = step)
	            : (previousNewFiber.sibling = step),
	          (previousNewFiber = step));
	    shouldTrackSideEffects &&
	      oldFiber.forEach(function (child) {
	        return deleteChild(returnFiber, child);
	      });
	    isHydrating && pushTreeFork(returnFiber, newIdx);
	    return resultingFirstChild;
	  }
	  function reconcileChildFibersImpl(
	    returnFiber,
	    currentFirstChild,
	    newChild,
	    lanes
	  ) {
	    "object" === typeof newChild &&
	      null !== newChild &&
	      newChild.type === REACT_FRAGMENT_TYPE &&
	      null === newChild.key &&
	      (newChild = newChild.props.children);
	    if ("object" === typeof newChild && null !== newChild) {
	      switch (newChild.$$typeof) {
	        case REACT_ELEMENT_TYPE:
	          a: {
	            for (var key = newChild.key; null !== currentFirstChild; ) {
	              if (currentFirstChild.key === key) {
	                key = newChild.type;
	                if (key === REACT_FRAGMENT_TYPE) {
	                  if (7 === currentFirstChild.tag) {
	                    deleteRemainingChildren(
	                      returnFiber,
	                      currentFirstChild.sibling
	                    );
	                    lanes = useFiber(
	                      currentFirstChild,
	                      newChild.props.children
	                    );
	                    lanes.return = returnFiber;
	                    returnFiber = lanes;
	                    break a;
	                  }
	                } else if (
	                  currentFirstChild.elementType === key ||
	                  ("object" === typeof key &&
	                    null !== key &&
	                    key.$$typeof === REACT_LAZY_TYPE &&
	                    resolveLazy(key) === currentFirstChild.type)
	                ) {
	                  deleteRemainingChildren(
	                    returnFiber,
	                    currentFirstChild.sibling
	                  );
	                  lanes = useFiber(currentFirstChild, newChild.props);
	                  coerceRef(lanes, newChild);
	                  lanes.return = returnFiber;
	                  returnFiber = lanes;
	                  break a;
	                }
	                deleteRemainingChildren(returnFiber, currentFirstChild);
	                break;
	              } else deleteChild(returnFiber, currentFirstChild);
	              currentFirstChild = currentFirstChild.sibling;
	            }
	            newChild.type === REACT_FRAGMENT_TYPE
	              ? ((lanes = createFiberFromFragment(
	                  newChild.props.children,
	                  returnFiber.mode,
	                  lanes,
	                  newChild.key
	                )),
	                (lanes.return = returnFiber),
	                (returnFiber = lanes))
	              : ((lanes = createFiberFromTypeAndProps(
	                  newChild.type,
	                  newChild.key,
	                  newChild.props,
	                  null,
	                  returnFiber.mode,
	                  lanes
	                )),
	                coerceRef(lanes, newChild),
	                (lanes.return = returnFiber),
	                (returnFiber = lanes));
	          }
	          return placeSingleChild(returnFiber);
	        case REACT_PORTAL_TYPE:
	          a: {
	            for (key = newChild.key; null !== currentFirstChild; ) {
	              if (currentFirstChild.key === key)
	                if (
	                  4 === currentFirstChild.tag &&
	                  currentFirstChild.stateNode.containerInfo ===
	                    newChild.containerInfo &&
	                  currentFirstChild.stateNode.implementation ===
	                    newChild.implementation
	                ) {
	                  deleteRemainingChildren(
	                    returnFiber,
	                    currentFirstChild.sibling
	                  );
	                  lanes = useFiber(currentFirstChild, newChild.children || []);
	                  lanes.return = returnFiber;
	                  returnFiber = lanes;
	                  break a;
	                } else {
	                  deleteRemainingChildren(returnFiber, currentFirstChild);
	                  break;
	                }
	              else deleteChild(returnFiber, currentFirstChild);
	              currentFirstChild = currentFirstChild.sibling;
	            }
	            lanes = createFiberFromPortal(newChild, returnFiber.mode, lanes);
	            lanes.return = returnFiber;
	            returnFiber = lanes;
	          }
	          return placeSingleChild(returnFiber);
	        case REACT_LAZY_TYPE:
	          return (
	            (newChild = resolveLazy(newChild)),
	            reconcileChildFibersImpl(
	              returnFiber,
	              currentFirstChild,
	              newChild,
	              lanes
	            )
	          );
	      }
	      if (isArrayImpl(newChild))
	        return reconcileChildrenArray(
	          returnFiber,
	          currentFirstChild,
	          newChild,
	          lanes
	        );
	      if (getIteratorFn(newChild)) {
	        key = getIteratorFn(newChild);
	        if ("function" !== typeof key) throw Error(formatProdErrorMessage(150));
	        newChild = key.call(newChild);
	        return reconcileChildrenIterator(
	          returnFiber,
	          currentFirstChild,
	          newChild,
	          lanes
	        );
	      }
	      if ("function" === typeof newChild.then)
	        return reconcileChildFibersImpl(
	          returnFiber,
	          currentFirstChild,
	          unwrapThenable(newChild),
	          lanes
	        );
	      if (newChild.$$typeof === REACT_CONTEXT_TYPE)
	        return reconcileChildFibersImpl(
	          returnFiber,
	          currentFirstChild,
	          readContextDuringReconciliation(returnFiber, newChild),
	          lanes
	        );
	      throwOnInvalidObjectTypeImpl(returnFiber, newChild);
	    }
	    return ("string" === typeof newChild && "" !== newChild) ||
	      "number" === typeof newChild ||
	      "bigint" === typeof newChild
	      ? ((newChild = "" + newChild),
	        null !== currentFirstChild && 6 === currentFirstChild.tag
	          ? (deleteRemainingChildren(returnFiber, currentFirstChild.sibling),
	            (lanes = useFiber(currentFirstChild, newChild)),
	            (lanes.return = returnFiber),
	            (returnFiber = lanes))
	          : (deleteRemainingChildren(returnFiber, currentFirstChild),
	            (lanes = createFiberFromText(newChild, returnFiber.mode, lanes)),
	            (lanes.return = returnFiber),
	            (returnFiber = lanes)),
	        placeSingleChild(returnFiber))
	      : deleteRemainingChildren(returnFiber, currentFirstChild);
	  }
	  return function (returnFiber, currentFirstChild, newChild, lanes) {
	    try {
	      thenableIndexCounter$1 = 0;
	      var firstChildFiber = reconcileChildFibersImpl(
	        returnFiber,
	        currentFirstChild,
	        newChild,
	        lanes
	      );
	      thenableState$1 = null;
	      return firstChildFiber;
	    } catch (x) {
	      if (x === SuspenseException || x === SuspenseActionException) throw x;
	      var fiber = createFiberImplClass(29, x, null, returnFiber.mode);
	      fiber.lanes = lanes;
	      fiber.return = returnFiber;
	      return fiber;
	    } finally {
	    }
	  };
	}
	var reconcileChildFibers = createChildReconciler(!0),
	  mountChildFibers = createChildReconciler(!1),
	  hasForceUpdate = !1;
	function initializeUpdateQueue(fiber) {
	  fiber.updateQueue = {
	    baseState: fiber.memoizedState,
	    firstBaseUpdate: null,
	    lastBaseUpdate: null,
	    shared: { pending: null, lanes: 0, hiddenCallbacks: null },
	    callbacks: null
	  };
	}
	function cloneUpdateQueue(current, workInProgress) {
	  current = current.updateQueue;
	  workInProgress.updateQueue === current &&
	    (workInProgress.updateQueue = {
	      baseState: current.baseState,
	      firstBaseUpdate: current.firstBaseUpdate,
	      lastBaseUpdate: current.lastBaseUpdate,
	      shared: current.shared,
	      callbacks: null
	    });
	}
	function createUpdate(lane) {
	  return { lane: lane, tag: 0, payload: null, callback: null, next: null };
	}
	function enqueueUpdate(fiber, update, lane) {
	  var updateQueue = fiber.updateQueue;
	  if (null === updateQueue) return null;
	  updateQueue = updateQueue.shared;
	  if (0 !== (executionContext & 2)) {
	    var pending = updateQueue.pending;
	    null === pending
	      ? (update.next = update)
	      : ((update.next = pending.next), (pending.next = update));
	    updateQueue.pending = update;
	    update = getRootForUpdatedFiber(fiber);
	    markUpdateLaneFromFiberToRoot(fiber, null, lane);
	    return update;
	  }
	  enqueueUpdate$1(fiber, updateQueue, update, lane);
	  return getRootForUpdatedFiber(fiber);
	}
	function entangleTransitions(root, fiber, lane) {
	  fiber = fiber.updateQueue;
	  if (null !== fiber && ((fiber = fiber.shared), 0 !== (lane & 4194048))) {
	    var queueLanes = fiber.lanes;
	    queueLanes &= root.pendingLanes;
	    lane |= queueLanes;
	    fiber.lanes = lane;
	    markRootEntangled(root, lane);
	  }
	}
	function enqueueCapturedUpdate(workInProgress, capturedUpdate) {
	  var queue = workInProgress.updateQueue,
	    current = workInProgress.alternate;
	  if (
	    null !== current &&
	    ((current = current.updateQueue), queue === current)
	  ) {
	    var newFirst = null,
	      newLast = null;
	    queue = queue.firstBaseUpdate;
	    if (null !== queue) {
	      do {
	        var clone = {
	          lane: queue.lane,
	          tag: queue.tag,
	          payload: queue.payload,
	          callback: null,
	          next: null
	        };
	        null === newLast
	          ? (newFirst = newLast = clone)
	          : (newLast = newLast.next = clone);
	        queue = queue.next;
	      } while (null !== queue);
	      null === newLast
	        ? (newFirst = newLast = capturedUpdate)
	        : (newLast = newLast.next = capturedUpdate);
	    } else newFirst = newLast = capturedUpdate;
	    queue = {
	      baseState: current.baseState,
	      firstBaseUpdate: newFirst,
	      lastBaseUpdate: newLast,
	      shared: current.shared,
	      callbacks: current.callbacks
	    };
	    workInProgress.updateQueue = queue;
	    return;
	  }
	  workInProgress = queue.lastBaseUpdate;
	  null === workInProgress
	    ? (queue.firstBaseUpdate = capturedUpdate)
	    : (workInProgress.next = capturedUpdate);
	  queue.lastBaseUpdate = capturedUpdate;
	}
	var didReadFromEntangledAsyncAction = !1;
	function suspendIfUpdateReadFromEntangledAsyncAction() {
	  if (didReadFromEntangledAsyncAction) {
	    var entangledActionThenable = currentEntangledActionThenable;
	    if (null !== entangledActionThenable) throw entangledActionThenable;
	  }
	}
	function processUpdateQueue(
	  workInProgress$jscomp$0,
	  props,
	  instance$jscomp$0,
	  renderLanes
	) {
	  didReadFromEntangledAsyncAction = !1;
	  var queue = workInProgress$jscomp$0.updateQueue;
	  hasForceUpdate = !1;
	  var firstBaseUpdate = queue.firstBaseUpdate,
	    lastBaseUpdate = queue.lastBaseUpdate,
	    pendingQueue = queue.shared.pending;
	  if (null !== pendingQueue) {
	    queue.shared.pending = null;
	    var lastPendingUpdate = pendingQueue,
	      firstPendingUpdate = lastPendingUpdate.next;
	    lastPendingUpdate.next = null;
	    null === lastBaseUpdate
	      ? (firstBaseUpdate = firstPendingUpdate)
	      : (lastBaseUpdate.next = firstPendingUpdate);
	    lastBaseUpdate = lastPendingUpdate;
	    var current = workInProgress$jscomp$0.alternate;
	    null !== current &&
	      ((current = current.updateQueue),
	      (pendingQueue = current.lastBaseUpdate),
	      pendingQueue !== lastBaseUpdate &&
	        (null === pendingQueue
	          ? (current.firstBaseUpdate = firstPendingUpdate)
	          : (pendingQueue.next = firstPendingUpdate),
	        (current.lastBaseUpdate = lastPendingUpdate)));
	  }
	  if (null !== firstBaseUpdate) {
	    var newState = queue.baseState;
	    lastBaseUpdate = 0;
	    current = firstPendingUpdate = lastPendingUpdate = null;
	    pendingQueue = firstBaseUpdate;
	    do {
	      var updateLane = pendingQueue.lane & -536870913,
	        isHiddenUpdate = updateLane !== pendingQueue.lane;
	      if (
	        isHiddenUpdate
	          ? (workInProgressRootRenderLanes & updateLane) === updateLane
	          : (renderLanes & updateLane) === updateLane
	      ) {
	        0 !== updateLane &&
	          updateLane === currentEntangledLane &&
	          (didReadFromEntangledAsyncAction = !0);
	        null !== current &&
	          (current = current.next =
	            {
	              lane: 0,
	              tag: pendingQueue.tag,
	              payload: pendingQueue.payload,
	              callback: null,
	              next: null
	            });
	        a: {
	          var workInProgress = workInProgress$jscomp$0,
	            update = pendingQueue;
	          updateLane = props;
	          var instance = instance$jscomp$0;
	          switch (update.tag) {
	            case 1:
	              workInProgress = update.payload;
	              if ("function" === typeof workInProgress) {
	                newState = workInProgress.call(instance, newState, updateLane);
	                break a;
	              }
	              newState = workInProgress;
	              break a;
	            case 3:
	              workInProgress.flags = (workInProgress.flags & -65537) | 128;
	            case 0:
	              workInProgress = update.payload;
	              updateLane =
	                "function" === typeof workInProgress
	                  ? workInProgress.call(instance, newState, updateLane)
	                  : workInProgress;
	              if (null === updateLane || void 0 === updateLane) break a;
	              newState = assign({}, newState, updateLane);
	              break a;
	            case 2:
	              hasForceUpdate = !0;
	          }
	        }
	        updateLane = pendingQueue.callback;
	        null !== updateLane &&
	          ((workInProgress$jscomp$0.flags |= 64),
	          isHiddenUpdate && (workInProgress$jscomp$0.flags |= 8192),
	          (isHiddenUpdate = queue.callbacks),
	          null === isHiddenUpdate
	            ? (queue.callbacks = [updateLane])
	            : isHiddenUpdate.push(updateLane));
	      } else
	        (isHiddenUpdate = {
	          lane: updateLane,
	          tag: pendingQueue.tag,
	          payload: pendingQueue.payload,
	          callback: pendingQueue.callback,
	          next: null
	        }),
	          null === current
	            ? ((firstPendingUpdate = current = isHiddenUpdate),
	              (lastPendingUpdate = newState))
	            : (current = current.next = isHiddenUpdate),
	          (lastBaseUpdate |= updateLane);
	      pendingQueue = pendingQueue.next;
	      if (null === pendingQueue)
	        if (((pendingQueue = queue.shared.pending), null === pendingQueue))
	          break;
	        else
	          (isHiddenUpdate = pendingQueue),
	            (pendingQueue = isHiddenUpdate.next),
	            (isHiddenUpdate.next = null),
	            (queue.lastBaseUpdate = isHiddenUpdate),
	            (queue.shared.pending = null);
	    } while (1);
	    null === current && (lastPendingUpdate = newState);
	    queue.baseState = lastPendingUpdate;
	    queue.firstBaseUpdate = firstPendingUpdate;
	    queue.lastBaseUpdate = current;
	    null === firstBaseUpdate && (queue.shared.lanes = 0);
	    workInProgressRootSkippedLanes |= lastBaseUpdate;
	    workInProgress$jscomp$0.lanes = lastBaseUpdate;
	    workInProgress$jscomp$0.memoizedState = newState;
	  }
	}
	function callCallback(callback, context) {
	  if ("function" !== typeof callback)
	    throw Error(formatProdErrorMessage(191, callback));
	  callback.call(context);
	}
	function commitCallbacks(updateQueue, context) {
	  var callbacks = updateQueue.callbacks;
	  if (null !== callbacks)
	    for (
	      updateQueue.callbacks = null, updateQueue = 0;
	      updateQueue < callbacks.length;
	      updateQueue++
	    )
	      callCallback(callbacks[updateQueue], context);
	}
	var currentTreeHiddenStackCursor = createCursor(null),
	  prevEntangledRenderLanesCursor = createCursor(0);
	function pushHiddenContext(fiber, context) {
	  fiber = entangledRenderLanes;
	  push(prevEntangledRenderLanesCursor, fiber);
	  push(currentTreeHiddenStackCursor, context);
	  entangledRenderLanes = fiber | context.baseLanes;
	}
	function reuseHiddenContextOnStack() {
	  push(prevEntangledRenderLanesCursor, entangledRenderLanes);
	  push(currentTreeHiddenStackCursor, currentTreeHiddenStackCursor.current);
	}
	function popHiddenContext() {
	  entangledRenderLanes = prevEntangledRenderLanesCursor.current;
	  pop(currentTreeHiddenStackCursor);
	  pop(prevEntangledRenderLanesCursor);
	}
	var suspenseHandlerStackCursor = createCursor(null),
	  shellBoundary = null;
	function pushPrimaryTreeSuspenseHandler(handler) {
	  var current = handler.alternate;
	  push(suspenseStackCursor, suspenseStackCursor.current & 1);
	  push(suspenseHandlerStackCursor, handler);
	  null === shellBoundary &&
	    (null === current || null !== currentTreeHiddenStackCursor.current
	      ? (shellBoundary = handler)
	      : null !== current.memoizedState && (shellBoundary = handler));
	}
	function pushDehydratedActivitySuspenseHandler(fiber) {
	  push(suspenseStackCursor, suspenseStackCursor.current);
	  push(suspenseHandlerStackCursor, fiber);
	  null === shellBoundary && (shellBoundary = fiber);
	}
	function pushOffscreenSuspenseHandler(fiber) {
	  22 === fiber.tag
	    ? (push(suspenseStackCursor, suspenseStackCursor.current),
	      push(suspenseHandlerStackCursor, fiber),
	      null === shellBoundary && (shellBoundary = fiber))
	    : reuseSuspenseHandlerOnStack(fiber);
	}
	function reuseSuspenseHandlerOnStack() {
	  push(suspenseStackCursor, suspenseStackCursor.current);
	  push(suspenseHandlerStackCursor, suspenseHandlerStackCursor.current);
	}
	function popSuspenseHandler(fiber) {
	  pop(suspenseHandlerStackCursor);
	  shellBoundary === fiber && (shellBoundary = null);
	  pop(suspenseStackCursor);
	}
	var suspenseStackCursor = createCursor(0);
	function findFirstSuspended(row) {
	  for (var node = row; null !== node; ) {
	    if (13 === node.tag) {
	      var state = node.memoizedState;
	      if (
	        null !== state &&
	        ((state = state.dehydrated),
	        null === state ||
	          isSuspenseInstancePending(state) ||
	          isSuspenseInstanceFallback(state))
	      )
	        return node;
	    } else if (
	      19 === node.tag &&
	      ("forwards" === node.memoizedProps.revealOrder ||
	        "backwards" === node.memoizedProps.revealOrder ||
	        "unstable_legacy-backwards" === node.memoizedProps.revealOrder ||
	        "together" === node.memoizedProps.revealOrder)
	    ) {
	      if (0 !== (node.flags & 128)) return node;
	    } else if (null !== node.child) {
	      node.child.return = node;
	      node = node.child;
	      continue;
	    }
	    if (node === row) break;
	    for (; null === node.sibling; ) {
	      if (null === node.return || node.return === row) return null;
	      node = node.return;
	    }
	    node.sibling.return = node.return;
	    node = node.sibling;
	  }
	  return null;
	}
	var renderLanes = 0,
	  currentlyRenderingFiber = null,
	  currentHook = null,
	  workInProgressHook = null,
	  didScheduleRenderPhaseUpdate = !1,
	  didScheduleRenderPhaseUpdateDuringThisPass = !1,
	  shouldDoubleInvokeUserFnsInHooksDEV = !1,
	  localIdCounter = 0,
	  thenableIndexCounter = 0,
	  thenableState = null,
	  globalClientIdCounter = 0;
	function throwInvalidHookError() {
	  throw Error(formatProdErrorMessage(321));
	}
	function areHookInputsEqual(nextDeps, prevDeps) {
	  if (null === prevDeps) return !1;
	  for (var i = 0; i < prevDeps.length && i < nextDeps.length; i++)
	    if (!objectIs(nextDeps[i], prevDeps[i])) return !1;
	  return !0;
	}
	function renderWithHooks(
	  current,
	  workInProgress,
	  Component,
	  props,
	  secondArg,
	  nextRenderLanes
	) {
	  renderLanes = nextRenderLanes;
	  currentlyRenderingFiber = workInProgress;
	  workInProgress.memoizedState = null;
	  workInProgress.updateQueue = null;
	  workInProgress.lanes = 0;
	  ReactSharedInternals.H =
	    null === current || null === current.memoizedState
	      ? HooksDispatcherOnMount
	      : HooksDispatcherOnUpdate;
	  shouldDoubleInvokeUserFnsInHooksDEV = !1;
	  nextRenderLanes = Component(props, secondArg);
	  shouldDoubleInvokeUserFnsInHooksDEV = !1;
	  didScheduleRenderPhaseUpdateDuringThisPass &&
	    (nextRenderLanes = renderWithHooksAgain(
	      workInProgress,
	      Component,
	      props,
	      secondArg
	    ));
	  finishRenderingHooks(current);
	  return nextRenderLanes;
	}
	function finishRenderingHooks(current) {
	  ReactSharedInternals.H = ContextOnlyDispatcher;
	  var didRenderTooFewHooks = null !== currentHook && null !== currentHook.next;
	  renderLanes = 0;
	  workInProgressHook = currentHook = currentlyRenderingFiber = null;
	  didScheduleRenderPhaseUpdate = !1;
	  thenableIndexCounter = 0;
	  thenableState = null;
	  if (didRenderTooFewHooks) throw Error(formatProdErrorMessage(300));
	  null === current ||
	    didReceiveUpdate ||
	    ((current = current.dependencies),
	    null !== current &&
	      checkIfContextChanged(current) &&
	      (didReceiveUpdate = !0));
	}
	function renderWithHooksAgain(workInProgress, Component, props, secondArg) {
	  currentlyRenderingFiber = workInProgress;
	  var numberOfReRenders = 0;
	  do {
	    didScheduleRenderPhaseUpdateDuringThisPass && (thenableState = null);
	    thenableIndexCounter = 0;
	    didScheduleRenderPhaseUpdateDuringThisPass = !1;
	    if (25 <= numberOfReRenders) throw Error(formatProdErrorMessage(301));
	    numberOfReRenders += 1;
	    workInProgressHook = currentHook = null;
	    if (null != workInProgress.updateQueue) {
	      var children = workInProgress.updateQueue;
	      children.lastEffect = null;
	      children.events = null;
	      children.stores = null;
	      null != children.memoCache && (children.memoCache.index = 0);
	    }
	    ReactSharedInternals.H = HooksDispatcherOnRerender;
	    children = Component(props, secondArg);
	  } while (didScheduleRenderPhaseUpdateDuringThisPass);
	  return children;
	}
	function TransitionAwareHostComponent() {
	  var dispatcher = ReactSharedInternals.H,
	    maybeThenable = dispatcher.useState()[0];
	  maybeThenable =
	    "function" === typeof maybeThenable.then
	      ? useThenable(maybeThenable)
	      : maybeThenable;
	  dispatcher = dispatcher.useState()[0];
	  (null !== currentHook ? currentHook.memoizedState : null) !== dispatcher &&
	    (currentlyRenderingFiber.flags |= 1024);
	  return maybeThenable;
	}
	function checkDidRenderIdHook() {
	  var didRenderIdHook = 0 !== localIdCounter;
	  localIdCounter = 0;
	  return didRenderIdHook;
	}
	function bailoutHooks(current, workInProgress, lanes) {
	  workInProgress.updateQueue = current.updateQueue;
	  workInProgress.flags &= -2053;
	  current.lanes &= ~lanes;
	}
	function resetHooksOnUnwind(workInProgress) {
	  if (didScheduleRenderPhaseUpdate) {
	    for (
	      workInProgress = workInProgress.memoizedState;
	      null !== workInProgress;

	    ) {
	      var queue = workInProgress.queue;
	      null !== queue && (queue.pending = null);
	      workInProgress = workInProgress.next;
	    }
	    didScheduleRenderPhaseUpdate = !1;
	  }
	  renderLanes = 0;
	  workInProgressHook = currentHook = currentlyRenderingFiber = null;
	  didScheduleRenderPhaseUpdateDuringThisPass = !1;
	  thenableIndexCounter = localIdCounter = 0;
	  thenableState = null;
	}
	function mountWorkInProgressHook() {
	  var hook = {
	    memoizedState: null,
	    baseState: null,
	    baseQueue: null,
	    queue: null,
	    next: null
	  };
	  null === workInProgressHook
	    ? (currentlyRenderingFiber.memoizedState = workInProgressHook = hook)
	    : (workInProgressHook = workInProgressHook.next = hook);
	  return workInProgressHook;
	}
	function updateWorkInProgressHook() {
	  if (null === currentHook) {
	    var nextCurrentHook = currentlyRenderingFiber.alternate;
	    nextCurrentHook =
	      null !== nextCurrentHook ? nextCurrentHook.memoizedState : null;
	  } else nextCurrentHook = currentHook.next;
	  var nextWorkInProgressHook =
	    null === workInProgressHook
	      ? currentlyRenderingFiber.memoizedState
	      : workInProgressHook.next;
	  if (null !== nextWorkInProgressHook)
	    (workInProgressHook = nextWorkInProgressHook),
	      (currentHook = nextCurrentHook);
	  else {
	    if (null === nextCurrentHook) {
	      if (null === currentlyRenderingFiber.alternate)
	        throw Error(formatProdErrorMessage(467));
	      throw Error(formatProdErrorMessage(310));
	    }
	    currentHook = nextCurrentHook;
	    nextCurrentHook = {
	      memoizedState: currentHook.memoizedState,
	      baseState: currentHook.baseState,
	      baseQueue: currentHook.baseQueue,
	      queue: currentHook.queue,
	      next: null
	    };
	    null === workInProgressHook
	      ? (currentlyRenderingFiber.memoizedState = workInProgressHook =
	          nextCurrentHook)
	      : (workInProgressHook = workInProgressHook.next = nextCurrentHook);
	  }
	  return workInProgressHook;
	}
	function createFunctionComponentUpdateQueue() {
	  return { lastEffect: null, events: null, stores: null, memoCache: null };
	}
	function useThenable(thenable) {
	  var index = thenableIndexCounter;
	  thenableIndexCounter += 1;
	  null === thenableState && (thenableState = []);
	  thenable = trackUsedThenable(thenableState, thenable, index);
	  index = currentlyRenderingFiber;
	  null ===
	    (null === workInProgressHook
	      ? index.memoizedState
	      : workInProgressHook.next) &&
	    ((index = index.alternate),
	    (ReactSharedInternals.H =
	      null === index || null === index.memoizedState
	        ? HooksDispatcherOnMount
	        : HooksDispatcherOnUpdate));
	  return thenable;
	}
	function use(usable) {
	  if (null !== usable && "object" === typeof usable) {
	    if ("function" === typeof usable.then) return useThenable(usable);
	    if (usable.$$typeof === REACT_CONTEXT_TYPE) return readContext(usable);
	  }
	  throw Error(formatProdErrorMessage(438, String(usable)));
	}
	function useMemoCache(size) {
	  var memoCache = null,
	    updateQueue = currentlyRenderingFiber.updateQueue;
	  null !== updateQueue && (memoCache = updateQueue.memoCache);
	  if (null == memoCache) {
	    var current = currentlyRenderingFiber.alternate;
	    null !== current &&
	      ((current = current.updateQueue),
	      null !== current &&
	        ((current = current.memoCache),
	        null != current &&
	          (memoCache = {
	            data: current.data.map(function (array) {
	              return array.slice();
	            }),
	            index: 0
	          })));
	  }
	  null == memoCache && (memoCache = { data: [], index: 0 });
	  null === updateQueue &&
	    ((updateQueue = createFunctionComponentUpdateQueue()),
	    (currentlyRenderingFiber.updateQueue = updateQueue));
	  updateQueue.memoCache = memoCache;
	  updateQueue = memoCache.data[memoCache.index];
	  if (void 0 === updateQueue)
	    for (
	      updateQueue = memoCache.data[memoCache.index] = Array(size), current = 0;
	      current < size;
	      current++
	    )
	      updateQueue[current] = REACT_MEMO_CACHE_SENTINEL;
	  memoCache.index++;
	  return updateQueue;
	}
	function basicStateReducer(state, action) {
	  return "function" === typeof action ? action(state) : action;
	}
	function updateReducer(reducer) {
	  var hook = updateWorkInProgressHook();
	  return updateReducerImpl(hook, currentHook, reducer);
	}
	function updateReducerImpl(hook, current, reducer) {
	  var queue = hook.queue;
	  if (null === queue) throw Error(formatProdErrorMessage(311));
	  queue.lastRenderedReducer = reducer;
	  var baseQueue = hook.baseQueue,
	    pendingQueue = queue.pending;
	  if (null !== pendingQueue) {
	    if (null !== baseQueue) {
	      var baseFirst = baseQueue.next;
	      baseQueue.next = pendingQueue.next;
	      pendingQueue.next = baseFirst;
	    }
	    current.baseQueue = baseQueue = pendingQueue;
	    queue.pending = null;
	  }
	  pendingQueue = hook.baseState;
	  if (null === baseQueue) hook.memoizedState = pendingQueue;
	  else {
	    current = baseQueue.next;
	    var newBaseQueueFirst = (baseFirst = null),
	      newBaseQueueLast = null,
	      update = current,
	      didReadFromEntangledAsyncAction$60 = !1;
	    do {
	      var updateLane = update.lane & -536870913;
	      if (
	        updateLane !== update.lane
	          ? (workInProgressRootRenderLanes & updateLane) === updateLane
	          : (renderLanes & updateLane) === updateLane
	      ) {
	        var revertLane = update.revertLane;
	        if (0 === revertLane)
	          null !== newBaseQueueLast &&
	            (newBaseQueueLast = newBaseQueueLast.next =
	              {
	                lane: 0,
	                revertLane: 0,
	                gesture: null,
	                action: update.action,
	                hasEagerState: update.hasEagerState,
	                eagerState: update.eagerState,
	                next: null
	              }),
	            updateLane === currentEntangledLane &&
	              (didReadFromEntangledAsyncAction$60 = !0);
	        else if ((renderLanes & revertLane) === revertLane) {
	          update = update.next;
	          revertLane === currentEntangledLane &&
	            (didReadFromEntangledAsyncAction$60 = !0);
	          continue;
	        } else
	          (updateLane = {
	            lane: 0,
	            revertLane: update.revertLane,
	            gesture: null,
	            action: update.action,
	            hasEagerState: update.hasEagerState,
	            eagerState: update.eagerState,
	            next: null
	          }),
	            null === newBaseQueueLast
	              ? ((newBaseQueueFirst = newBaseQueueLast = updateLane),
	                (baseFirst = pendingQueue))
	              : (newBaseQueueLast = newBaseQueueLast.next = updateLane),
	            (currentlyRenderingFiber.lanes |= revertLane),
	            (workInProgressRootSkippedLanes |= revertLane);
	        updateLane = update.action;
	        shouldDoubleInvokeUserFnsInHooksDEV &&
	          reducer(pendingQueue, updateLane);
	        pendingQueue = update.hasEagerState
	          ? update.eagerState
	          : reducer(pendingQueue, updateLane);
	      } else
	        (revertLane = {
	          lane: updateLane,
	          revertLane: update.revertLane,
	          gesture: update.gesture,
	          action: update.action,
	          hasEagerState: update.hasEagerState,
	          eagerState: update.eagerState,
	          next: null
	        }),
	          null === newBaseQueueLast
	            ? ((newBaseQueueFirst = newBaseQueueLast = revertLane),
	              (baseFirst = pendingQueue))
	            : (newBaseQueueLast = newBaseQueueLast.next = revertLane),
	          (currentlyRenderingFiber.lanes |= updateLane),
	          (workInProgressRootSkippedLanes |= updateLane);
	      update = update.next;
	    } while (null !== update && update !== current);
	    null === newBaseQueueLast
	      ? (baseFirst = pendingQueue)
	      : (newBaseQueueLast.next = newBaseQueueFirst);
	    if (
	      !objectIs(pendingQueue, hook.memoizedState) &&
	      ((didReceiveUpdate = !0),
	      didReadFromEntangledAsyncAction$60 &&
	        ((reducer = currentEntangledActionThenable), null !== reducer))
	    )
	      throw reducer;
	    hook.memoizedState = pendingQueue;
	    hook.baseState = baseFirst;
	    hook.baseQueue = newBaseQueueLast;
	    queue.lastRenderedState = pendingQueue;
	  }
	  null === baseQueue && (queue.lanes = 0);
	  return [hook.memoizedState, queue.dispatch];
	}
	function rerenderReducer(reducer) {
	  var hook = updateWorkInProgressHook(),
	    queue = hook.queue;
	  if (null === queue) throw Error(formatProdErrorMessage(311));
	  queue.lastRenderedReducer = reducer;
	  var dispatch = queue.dispatch,
	    lastRenderPhaseUpdate = queue.pending,
	    newState = hook.memoizedState;
	  if (null !== lastRenderPhaseUpdate) {
	    queue.pending = null;
	    var update = (lastRenderPhaseUpdate = lastRenderPhaseUpdate.next);
	    do (newState = reducer(newState, update.action)), (update = update.next);
	    while (update !== lastRenderPhaseUpdate);
	    objectIs(newState, hook.memoizedState) || (didReceiveUpdate = !0);
	    hook.memoizedState = newState;
	    null === hook.baseQueue && (hook.baseState = newState);
	    queue.lastRenderedState = newState;
	  }
	  return [newState, dispatch];
	}
	function updateSyncExternalStore(subscribe, getSnapshot, getServerSnapshot) {
	  var fiber = currentlyRenderingFiber,
	    hook = updateWorkInProgressHook(),
	    isHydrating$jscomp$0 = isHydrating;
	  if (isHydrating$jscomp$0) {
	    if (void 0 === getServerSnapshot) throw Error(formatProdErrorMessage(407));
	    getServerSnapshot = getServerSnapshot();
	  } else getServerSnapshot = getSnapshot();
	  var snapshotChanged = !objectIs(
	    (currentHook || hook).memoizedState,
	    getServerSnapshot
	  );
	  snapshotChanged &&
	    ((hook.memoizedState = getServerSnapshot), (didReceiveUpdate = !0));
	  hook = hook.queue;
	  updateEffect(subscribeToStore.bind(null, fiber, hook, subscribe), [
	    subscribe
	  ]);
	  if (
	    hook.getSnapshot !== getSnapshot ||
	    snapshotChanged ||
	    (null !== workInProgressHook && workInProgressHook.memoizedState.tag & 1)
	  ) {
	    fiber.flags |= 2048;
	    pushSimpleEffect(
	      9,
	      { destroy: void 0 },
	      updateStoreInstance.bind(
	        null,
	        fiber,
	        hook,
	        getServerSnapshot,
	        getSnapshot
	      ),
	      null
	    );
	    if (null === workInProgressRoot) throw Error(formatProdErrorMessage(349));
	    isHydrating$jscomp$0 ||
	      0 !== (renderLanes & 127) ||
	      pushStoreConsistencyCheck(fiber, getSnapshot, getServerSnapshot);
	  }
	  return getServerSnapshot;
	}
	function pushStoreConsistencyCheck(fiber, getSnapshot, renderedSnapshot) {
	  fiber.flags |= 16384;
	  fiber = { getSnapshot: getSnapshot, value: renderedSnapshot };
	  getSnapshot = currentlyRenderingFiber.updateQueue;
	  null === getSnapshot
	    ? ((getSnapshot = createFunctionComponentUpdateQueue()),
	      (currentlyRenderingFiber.updateQueue = getSnapshot),
	      (getSnapshot.stores = [fiber]))
	    : ((renderedSnapshot = getSnapshot.stores),
	      null === renderedSnapshot
	        ? (getSnapshot.stores = [fiber])
	        : renderedSnapshot.push(fiber));
	}
	function updateStoreInstance(fiber, inst, nextSnapshot, getSnapshot) {
	  inst.value = nextSnapshot;
	  inst.getSnapshot = getSnapshot;
	  checkIfSnapshotChanged(inst) && forceStoreRerender(fiber);
	}
	function subscribeToStore(fiber, inst, subscribe) {
	  return subscribe(function () {
	    checkIfSnapshotChanged(inst) && forceStoreRerender(fiber);
	  });
	}
	function checkIfSnapshotChanged(inst) {
	  var latestGetSnapshot = inst.getSnapshot;
	  inst = inst.value;
	  try {
	    var nextValue = latestGetSnapshot();
	    return !objectIs(inst, nextValue);
	  } catch (error) {
	    return !0;
	  }
	}
	function forceStoreRerender(fiber) {
	  var root = enqueueConcurrentRenderForLane(fiber, 2);
	  null !== root && scheduleUpdateOnFiber(root, fiber, 2);
	}
	function mountStateImpl(initialState) {
	  var hook = mountWorkInProgressHook();
	  if ("function" === typeof initialState) {
	    var initialStateInitializer = initialState;
	    initialState = initialStateInitializer();
	    if (shouldDoubleInvokeUserFnsInHooksDEV) {
	      setIsStrictModeForDevtools(!0);
	      try {
	        initialStateInitializer();
	      } finally {
	        setIsStrictModeForDevtools(!1);
	      }
	    }
	  }
	  hook.memoizedState = hook.baseState = initialState;
	  hook.queue = {
	    pending: null,
	    lanes: 0,
	    dispatch: null,
	    lastRenderedReducer: basicStateReducer,
	    lastRenderedState: initialState
	  };
	  return hook;
	}
	function updateOptimisticImpl(hook, current, passthrough, reducer) {
	  hook.baseState = passthrough;
	  return updateReducerImpl(
	    hook,
	    currentHook,
	    "function" === typeof reducer ? reducer : basicStateReducer
	  );
	}
	function dispatchActionState(
	  fiber,
	  actionQueue,
	  setPendingState,
	  setState,
	  payload
	) {
	  if (isRenderPhaseUpdate(fiber)) throw Error(formatProdErrorMessage(485));
	  fiber = actionQueue.action;
	  if (null !== fiber) {
	    var actionNode = {
	      payload: payload,
	      action: fiber,
	      next: null,
	      isTransition: !0,
	      status: "pending",
	      value: null,
	      reason: null,
	      listeners: [],
	      then: function (listener) {
	        actionNode.listeners.push(listener);
	      }
	    };
	    null !== ReactSharedInternals.T
	      ? setPendingState(!0)
	      : (actionNode.isTransition = !1);
	    setState(actionNode);
	    setPendingState = actionQueue.pending;
	    null === setPendingState
	      ? ((actionNode.next = actionQueue.pending = actionNode),
	        runActionStateAction(actionQueue, actionNode))
	      : ((actionNode.next = setPendingState.next),
	        (actionQueue.pending = setPendingState.next = actionNode));
	  }
	}
	function runActionStateAction(actionQueue, node) {
	  var action = node.action,
	    payload = node.payload,
	    prevState = actionQueue.state;
	  if (node.isTransition) {
	    var prevTransition = ReactSharedInternals.T,
	      currentTransition = {};
	    ReactSharedInternals.T = currentTransition;
	    try {
	      var returnValue = action(prevState, payload),
	        onStartTransitionFinish = ReactSharedInternals.S;
	      null !== onStartTransitionFinish &&
	        onStartTransitionFinish(currentTransition, returnValue);
	      handleActionReturnValue(actionQueue, node, returnValue);
	    } catch (error) {
	      onActionError(actionQueue, node, error);
	    } finally {
	      null !== prevTransition &&
	        null !== currentTransition.types &&
	        (prevTransition.types = currentTransition.types),
	        (ReactSharedInternals.T = prevTransition);
	    }
	  } else
	    try {
	      (prevTransition = action(prevState, payload)),
	        handleActionReturnValue(actionQueue, node, prevTransition);
	    } catch (error$66) {
	      onActionError(actionQueue, node, error$66);
	    }
	}
	function handleActionReturnValue(actionQueue, node, returnValue) {
	  null !== returnValue &&
	  "object" === typeof returnValue &&
	  "function" === typeof returnValue.then
	    ? returnValue.then(
	        function (nextState) {
	          onActionSuccess(actionQueue, node, nextState);
	        },
	        function (error) {
	          return onActionError(actionQueue, node, error);
	        }
	      )
	    : onActionSuccess(actionQueue, node, returnValue);
	}
	function onActionSuccess(actionQueue, actionNode, nextState) {
	  actionNode.status = "fulfilled";
	  actionNode.value = nextState;
	  notifyActionListeners(actionNode);
	  actionQueue.state = nextState;
	  actionNode = actionQueue.pending;
	  null !== actionNode &&
	    ((nextState = actionNode.next),
	    nextState === actionNode
	      ? (actionQueue.pending = null)
	      : ((nextState = nextState.next),
	        (actionNode.next = nextState),
	        runActionStateAction(actionQueue, nextState)));
	}
	function onActionError(actionQueue, actionNode, error) {
	  var last = actionQueue.pending;
	  actionQueue.pending = null;
	  if (null !== last) {
	    last = last.next;
	    do
	      (actionNode.status = "rejected"),
	        (actionNode.reason = error),
	        notifyActionListeners(actionNode),
	        (actionNode = actionNode.next);
	    while (actionNode !== last);
	  }
	  actionQueue.action = null;
	}
	function notifyActionListeners(actionNode) {
	  actionNode = actionNode.listeners;
	  for (var i = 0; i < actionNode.length; i++) (0, actionNode[i])();
	}
	function actionStateReducer(oldState, newState) {
	  return newState;
	}
	function mountActionState(action, initialStateProp) {
	  if (isHydrating) {
	    var ssrFormState = workInProgressRoot.formState;
	    if (null !== ssrFormState) {
	      a: {
	        var JSCompiler_inline_result = currentlyRenderingFiber;
	        if (isHydrating) {
	          if (nextHydratableInstance) {
	            b: {
	              var JSCompiler_inline_result$jscomp$0 = nextHydratableInstance;
	              for (
	                var inRootOrSingleton = rootOrSingletonContext;
	                8 !== JSCompiler_inline_result$jscomp$0.nodeType;

	              ) {
	                if (!inRootOrSingleton) {
	                  JSCompiler_inline_result$jscomp$0 = null;
	                  break b;
	                }
	                JSCompiler_inline_result$jscomp$0 = getNextHydratable(
	                  JSCompiler_inline_result$jscomp$0.nextSibling
	                );
	                if (null === JSCompiler_inline_result$jscomp$0) {
	                  JSCompiler_inline_result$jscomp$0 = null;
	                  break b;
	                }
	              }
	              inRootOrSingleton = JSCompiler_inline_result$jscomp$0.data;
	              JSCompiler_inline_result$jscomp$0 =
	                "F!" === inRootOrSingleton || "F" === inRootOrSingleton
	                  ? JSCompiler_inline_result$jscomp$0
	                  : null;
	            }
	            if (JSCompiler_inline_result$jscomp$0) {
	              nextHydratableInstance = getNextHydratable(
	                JSCompiler_inline_result$jscomp$0.nextSibling
	              );
	              JSCompiler_inline_result =
	                "F!" === JSCompiler_inline_result$jscomp$0.data;
	              break a;
	            }
	          }
	          throwOnHydrationMismatch(JSCompiler_inline_result);
	        }
	        JSCompiler_inline_result = !1;
	      }
	      JSCompiler_inline_result && (initialStateProp = ssrFormState[0]);
	    }
	  }
	  ssrFormState = mountWorkInProgressHook();
	  ssrFormState.memoizedState = ssrFormState.baseState = initialStateProp;
	  JSCompiler_inline_result = {
	    pending: null,
	    lanes: 0,
	    dispatch: null,
	    lastRenderedReducer: actionStateReducer,
	    lastRenderedState: initialStateProp
	  };
	  ssrFormState.queue = JSCompiler_inline_result;
	  ssrFormState = dispatchSetState.bind(
	    null,
	    currentlyRenderingFiber,
	    JSCompiler_inline_result
	  );
	  JSCompiler_inline_result.dispatch = ssrFormState;
	  JSCompiler_inline_result = mountStateImpl(!1);
	  inRootOrSingleton = dispatchOptimisticSetState.bind(
	    null,
	    currentlyRenderingFiber,
	    !1,
	    JSCompiler_inline_result.queue
	  );
	  JSCompiler_inline_result = mountWorkInProgressHook();
	  JSCompiler_inline_result$jscomp$0 = {
	    state: initialStateProp,
	    dispatch: null,
	    action: action,
	    pending: null
	  };
	  JSCompiler_inline_result.queue = JSCompiler_inline_result$jscomp$0;
	  ssrFormState = dispatchActionState.bind(
	    null,
	    currentlyRenderingFiber,
	    JSCompiler_inline_result$jscomp$0,
	    inRootOrSingleton,
	    ssrFormState
	  );
	  JSCompiler_inline_result$jscomp$0.dispatch = ssrFormState;
	  JSCompiler_inline_result.memoizedState = action;
	  return [initialStateProp, ssrFormState, !1];
	}
	function updateActionState(action) {
	  var stateHook = updateWorkInProgressHook();
	  return updateActionStateImpl(stateHook, currentHook, action);
	}
	function updateActionStateImpl(stateHook, currentStateHook, action) {
	  currentStateHook = updateReducerImpl(
	    stateHook,
	    currentStateHook,
	    actionStateReducer
	  )[0];
	  stateHook = updateReducer(basicStateReducer)[0];
	  if (
	    "object" === typeof currentStateHook &&
	    null !== currentStateHook &&
	    "function" === typeof currentStateHook.then
	  )
	    try {
	      var state = useThenable(currentStateHook);
	    } catch (x) {
	      if (x === SuspenseException) throw SuspenseActionException;
	      throw x;
	    }
	  else state = currentStateHook;
	  currentStateHook = updateWorkInProgressHook();
	  var actionQueue = currentStateHook.queue,
	    dispatch = actionQueue.dispatch;
	  action !== currentStateHook.memoizedState &&
	    ((currentlyRenderingFiber.flags |= 2048),
	    pushSimpleEffect(
	      9,
	      { destroy: void 0 },
	      actionStateActionEffect.bind(null, actionQueue, action),
	      null
	    ));
	  return [state, dispatch, stateHook];
	}
	function actionStateActionEffect(actionQueue, action) {
	  actionQueue.action = action;
	}
	function rerenderActionState(action) {
	  var stateHook = updateWorkInProgressHook(),
	    currentStateHook = currentHook;
	  if (null !== currentStateHook)
	    return updateActionStateImpl(stateHook, currentStateHook, action);
	  updateWorkInProgressHook();
	  stateHook = stateHook.memoizedState;
	  currentStateHook = updateWorkInProgressHook();
	  var dispatch = currentStateHook.queue.dispatch;
	  currentStateHook.memoizedState = action;
	  return [stateHook, dispatch, !1];
	}
	function pushSimpleEffect(tag, inst, create, deps) {
	  tag = { tag: tag, create: create, deps: deps, inst: inst, next: null };
	  inst = currentlyRenderingFiber.updateQueue;
	  null === inst &&
	    ((inst = createFunctionComponentUpdateQueue()),
	    (currentlyRenderingFiber.updateQueue = inst));
	  create = inst.lastEffect;
	  null === create
	    ? (inst.lastEffect = tag.next = tag)
	    : ((deps = create.next),
	      (create.next = tag),
	      (tag.next = deps),
	      (inst.lastEffect = tag));
	  return tag;
	}
	function updateRef() {
	  return updateWorkInProgressHook().memoizedState;
	}
	function mountEffectImpl(fiberFlags, hookFlags, create, deps) {
	  var hook = mountWorkInProgressHook();
	  currentlyRenderingFiber.flags |= fiberFlags;
	  hook.memoizedState = pushSimpleEffect(
	    1 | hookFlags,
	    { destroy: void 0 },
	    create,
	    void 0 === deps ? null : deps
	  );
	}
	function updateEffectImpl(fiberFlags, hookFlags, create, deps) {
	  var hook = updateWorkInProgressHook();
	  deps = void 0 === deps ? null : deps;
	  var inst = hook.memoizedState.inst;
	  null !== currentHook &&
	  null !== deps &&
	  areHookInputsEqual(deps, currentHook.memoizedState.deps)
	    ? (hook.memoizedState = pushSimpleEffect(hookFlags, inst, create, deps))
	    : ((currentlyRenderingFiber.flags |= fiberFlags),
	      (hook.memoizedState = pushSimpleEffect(
	        1 | hookFlags,
	        inst,
	        create,
	        deps
	      )));
	}
	function mountEffect(create, deps) {
	  mountEffectImpl(8390656, 8, create, deps);
	}
	function updateEffect(create, deps) {
	  updateEffectImpl(2048, 8, create, deps);
	}
	function useEffectEventImpl(payload) {
	  currentlyRenderingFiber.flags |= 4;
	  var componentUpdateQueue = currentlyRenderingFiber.updateQueue;
	  if (null === componentUpdateQueue)
	    (componentUpdateQueue = createFunctionComponentUpdateQueue()),
	      (currentlyRenderingFiber.updateQueue = componentUpdateQueue),
	      (componentUpdateQueue.events = [payload]);
	  else {
	    var events = componentUpdateQueue.events;
	    null === events
	      ? (componentUpdateQueue.events = [payload])
	      : events.push(payload);
	  }
	}
	function updateEvent(callback) {
	  var ref = updateWorkInProgressHook().memoizedState;
	  useEffectEventImpl({ ref: ref, nextImpl: callback });
	  return function () {
	    if (0 !== (executionContext & 2)) throw Error(formatProdErrorMessage(440));
	    return ref.impl.apply(void 0, arguments);
	  };
	}
	function updateInsertionEffect(create, deps) {
	  return updateEffectImpl(4, 2, create, deps);
	}
	function updateLayoutEffect(create, deps) {
	  return updateEffectImpl(4, 4, create, deps);
	}
	function imperativeHandleEffect(create, ref) {
	  if ("function" === typeof ref) {
	    create = create();
	    var refCleanup = ref(create);
	    return function () {
	      "function" === typeof refCleanup ? refCleanup() : ref(null);
	    };
	  }
	  if (null !== ref && void 0 !== ref)
	    return (
	      (create = create()),
	      (ref.current = create),
	      function () {
	        ref.current = null;
	      }
	    );
	}
	function updateImperativeHandle(ref, create, deps) {
	  deps = null !== deps && void 0 !== deps ? deps.concat([ref]) : null;
	  updateEffectImpl(4, 4, imperativeHandleEffect.bind(null, create, ref), deps);
	}
	function mountDebugValue() {}
	function updateCallback(callback, deps) {
	  var hook = updateWorkInProgressHook();
	  deps = void 0 === deps ? null : deps;
	  var prevState = hook.memoizedState;
	  if (null !== deps && areHookInputsEqual(deps, prevState[1]))
	    return prevState[0];
	  hook.memoizedState = [callback, deps];
	  return callback;
	}
	function updateMemo(nextCreate, deps) {
	  var hook = updateWorkInProgressHook();
	  deps = void 0 === deps ? null : deps;
	  var prevState = hook.memoizedState;
	  if (null !== deps && areHookInputsEqual(deps, prevState[1]))
	    return prevState[0];
	  prevState = nextCreate();
	  if (shouldDoubleInvokeUserFnsInHooksDEV) {
	    setIsStrictModeForDevtools(!0);
	    try {
	      nextCreate();
	    } finally {
	      setIsStrictModeForDevtools(!1);
	    }
	  }
	  hook.memoizedState = [prevState, deps];
	  return prevState;
	}
	function mountDeferredValueImpl(hook, value, initialValue) {
	  if (
	    void 0 === initialValue ||
	    (0 !== (renderLanes & 1073741824) &&
	      0 === (workInProgressRootRenderLanes & 261930))
	  )
	    return (hook.memoizedState = value);
	  hook.memoizedState = initialValue;
	  hook = requestDeferredLane();
	  currentlyRenderingFiber.lanes |= hook;
	  workInProgressRootSkippedLanes |= hook;
	  return initialValue;
	}
	function updateDeferredValueImpl(hook, prevValue, value, initialValue) {
	  if (objectIs(value, prevValue)) return value;
	  if (null !== currentTreeHiddenStackCursor.current)
	    return (
	      (hook = mountDeferredValueImpl(hook, value, initialValue)),
	      objectIs(hook, prevValue) || (didReceiveUpdate = !0),
	      hook
	    );
	  if (
	    0 === (renderLanes & 42) ||
	    (0 !== (renderLanes & 1073741824) &&
	      0 === (workInProgressRootRenderLanes & 261930))
	  )
	    return (didReceiveUpdate = !0), (hook.memoizedState = value);
	  hook = requestDeferredLane();
	  currentlyRenderingFiber.lanes |= hook;
	  workInProgressRootSkippedLanes |= hook;
	  return prevValue;
	}
	function startTransition(fiber, queue, pendingState, finishedState, callback) {
	  var previousPriority = ReactDOMSharedInternals.p;
	  ReactDOMSharedInternals.p =
	    0 !== previousPriority && 8 > previousPriority ? previousPriority : 8;
	  var prevTransition = ReactSharedInternals.T,
	    currentTransition = {};
	  ReactSharedInternals.T = currentTransition;
	  dispatchOptimisticSetState(fiber, !1, queue, pendingState);
	  try {
	    var returnValue = callback(),
	      onStartTransitionFinish = ReactSharedInternals.S;
	    null !== onStartTransitionFinish &&
	      onStartTransitionFinish(currentTransition, returnValue);
	    if (
	      null !== returnValue &&
	      "object" === typeof returnValue &&
	      "function" === typeof returnValue.then
	    ) {
	      var thenableForFinishedState = chainThenableValue(
	        returnValue,
	        finishedState
	      );
	      dispatchSetStateInternal(
	        fiber,
	        queue,
	        thenableForFinishedState,
	        requestUpdateLane(fiber)
	      );
	    } else
	      dispatchSetStateInternal(
	        fiber,
	        queue,
	        finishedState,
	        requestUpdateLane(fiber)
	      );
	  } catch (error) {
	    dispatchSetStateInternal(
	      fiber,
	      queue,
	      { then: function () {}, status: "rejected", reason: error },
	      requestUpdateLane()
	    );
	  } finally {
	    (ReactDOMSharedInternals.p = previousPriority),
	      null !== prevTransition &&
	        null !== currentTransition.types &&
	        (prevTransition.types = currentTransition.types),
	      (ReactSharedInternals.T = prevTransition);
	  }
	}
	function noop() {}
	function startHostTransition(formFiber, pendingState, action, formData) {
	  if (5 !== formFiber.tag) throw Error(formatProdErrorMessage(476));
	  var queue = ensureFormComponentIsStateful(formFiber).queue;
	  startTransition(
	    formFiber,
	    queue,
	    pendingState,
	    sharedNotPendingObject,
	    null === action
	      ? noop
	      : function () {
	          requestFormReset$1(formFiber);
	          return action(formData);
	        }
	  );
	}
	function ensureFormComponentIsStateful(formFiber) {
	  var existingStateHook = formFiber.memoizedState;
	  if (null !== existingStateHook) return existingStateHook;
	  existingStateHook = {
	    memoizedState: sharedNotPendingObject,
	    baseState: sharedNotPendingObject,
	    baseQueue: null,
	    queue: {
	      pending: null,
	      lanes: 0,
	      dispatch: null,
	      lastRenderedReducer: basicStateReducer,
	      lastRenderedState: sharedNotPendingObject
	    },
	    next: null
	  };
	  var initialResetState = {};
	  existingStateHook.next = {
	    memoizedState: initialResetState,
	    baseState: initialResetState,
	    baseQueue: null,
	    queue: {
	      pending: null,
	      lanes: 0,
	      dispatch: null,
	      lastRenderedReducer: basicStateReducer,
	      lastRenderedState: initialResetState
	    },
	    next: null
	  };
	  formFiber.memoizedState = existingStateHook;
	  formFiber = formFiber.alternate;
	  null !== formFiber && (formFiber.memoizedState = existingStateHook);
	  return existingStateHook;
	}
	function requestFormReset$1(formFiber) {
	  var stateHook = ensureFormComponentIsStateful(formFiber);
	  null === stateHook.next && (stateHook = formFiber.alternate.memoizedState);
	  dispatchSetStateInternal(
	    formFiber,
	    stateHook.next.queue,
	    {},
	    requestUpdateLane()
	  );
	}
	function useHostTransitionStatus() {
	  return readContext(HostTransitionContext);
	}
	function updateId() {
	  return updateWorkInProgressHook().memoizedState;
	}
	function updateRefresh() {
	  return updateWorkInProgressHook().memoizedState;
	}
	function refreshCache(fiber) {
	  for (var provider = fiber.return; null !== provider; ) {
	    switch (provider.tag) {
	      case 24:
	      case 3:
	        var lane = requestUpdateLane();
	        fiber = createUpdate(lane);
	        var root$69 = enqueueUpdate(provider, fiber, lane);
	        null !== root$69 &&
	          (scheduleUpdateOnFiber(root$69, provider, lane),
	          entangleTransitions(root$69, provider, lane));
	        provider = { cache: createCache() };
	        fiber.payload = provider;
	        return;
	    }
	    provider = provider.return;
	  }
	}
	function dispatchReducerAction(fiber, queue, action) {
	  var lane = requestUpdateLane();
	  action = {
	    lane: lane,
	    revertLane: 0,
	    gesture: null,
	    action: action,
	    hasEagerState: !1,
	    eagerState: null,
	    next: null
	  };
	  isRenderPhaseUpdate(fiber)
	    ? enqueueRenderPhaseUpdate(queue, action)
	    : ((action = enqueueConcurrentHookUpdate(fiber, queue, action, lane)),
	      null !== action &&
	        (scheduleUpdateOnFiber(action, fiber, lane),
	        entangleTransitionUpdate(action, queue, lane)));
	}
	function dispatchSetState(fiber, queue, action) {
	  var lane = requestUpdateLane();
	  dispatchSetStateInternal(fiber, queue, action, lane);
	}
	function dispatchSetStateInternal(fiber, queue, action, lane) {
	  var update = {
	    lane: lane,
	    revertLane: 0,
	    gesture: null,
	    action: action,
	    hasEagerState: !1,
	    eagerState: null,
	    next: null
	  };
	  if (isRenderPhaseUpdate(fiber)) enqueueRenderPhaseUpdate(queue, update);
	  else {
	    var alternate = fiber.alternate;
	    if (
	      0 === fiber.lanes &&
	      (null === alternate || 0 === alternate.lanes) &&
	      ((alternate = queue.lastRenderedReducer), null !== alternate)
	    )
	      try {
	        var currentState = queue.lastRenderedState,
	          eagerState = alternate(currentState, action);
	        update.hasEagerState = !0;
	        update.eagerState = eagerState;
	        if (objectIs(eagerState, currentState))
	          return (
	            enqueueUpdate$1(fiber, queue, update, 0),
	            null === workInProgressRoot && finishQueueingConcurrentUpdates(),
	            !1
	          );
	      } catch (error) {
	      } finally {
	      }
	    action = enqueueConcurrentHookUpdate(fiber, queue, update, lane);
	    if (null !== action)
	      return (
	        scheduleUpdateOnFiber(action, fiber, lane),
	        entangleTransitionUpdate(action, queue, lane),
	        !0
	      );
	  }
	  return !1;
	}
	function dispatchOptimisticSetState(fiber, throwIfDuringRender, queue, action) {
	  action = {
	    lane: 2,
	    revertLane: requestTransitionLane(),
	    gesture: null,
	    action: action,
	    hasEagerState: !1,
	    eagerState: null,
	    next: null
	  };
	  if (isRenderPhaseUpdate(fiber)) {
	    if (throwIfDuringRender) throw Error(formatProdErrorMessage(479));
	  } else
	    (throwIfDuringRender = enqueueConcurrentHookUpdate(
	      fiber,
	      queue,
	      action,
	      2
	    )),
	      null !== throwIfDuringRender &&
	        scheduleUpdateOnFiber(throwIfDuringRender, fiber, 2);
	}
	function isRenderPhaseUpdate(fiber) {
	  var alternate = fiber.alternate;
	  return (
	    fiber === currentlyRenderingFiber ||
	    (null !== alternate && alternate === currentlyRenderingFiber)
	  );
	}
	function enqueueRenderPhaseUpdate(queue, update) {
	  didScheduleRenderPhaseUpdateDuringThisPass = didScheduleRenderPhaseUpdate =
	    !0;
	  var pending = queue.pending;
	  null === pending
	    ? (update.next = update)
	    : ((update.next = pending.next), (pending.next = update));
	  queue.pending = update;
	}
	function entangleTransitionUpdate(root, queue, lane) {
	  if (0 !== (lane & 4194048)) {
	    var queueLanes = queue.lanes;
	    queueLanes &= root.pendingLanes;
	    lane |= queueLanes;
	    queue.lanes = lane;
	    markRootEntangled(root, lane);
	  }
	}
	var ContextOnlyDispatcher = {
	  readContext: readContext,
	  use: use,
	  useCallback: throwInvalidHookError,
	  useContext: throwInvalidHookError,
	  useEffect: throwInvalidHookError,
	  useImperativeHandle: throwInvalidHookError,
	  useLayoutEffect: throwInvalidHookError,
	  useInsertionEffect: throwInvalidHookError,
	  useMemo: throwInvalidHookError,
	  useReducer: throwInvalidHookError,
	  useRef: throwInvalidHookError,
	  useState: throwInvalidHookError,
	  useDebugValue: throwInvalidHookError,
	  useDeferredValue: throwInvalidHookError,
	  useTransition: throwInvalidHookError,
	  useSyncExternalStore: throwInvalidHookError,
	  useId: throwInvalidHookError,
	  useHostTransitionStatus: throwInvalidHookError,
	  useFormState: throwInvalidHookError,
	  useActionState: throwInvalidHookError,
	  useOptimistic: throwInvalidHookError,
	  useMemoCache: throwInvalidHookError,
	  useCacheRefresh: throwInvalidHookError
	};
	ContextOnlyDispatcher.useEffectEvent = throwInvalidHookError;
	var HooksDispatcherOnMount = {
	    readContext: readContext,
	    use: use,
	    useCallback: function (callback, deps) {
	      mountWorkInProgressHook().memoizedState = [
	        callback,
	        void 0 === deps ? null : deps
	      ];
	      return callback;
	    },
	    useContext: readContext,
	    useEffect: mountEffect,
	    useImperativeHandle: function (ref, create, deps) {
	      deps = null !== deps && void 0 !== deps ? deps.concat([ref]) : null;
	      mountEffectImpl(
	        4194308,
	        4,
	        imperativeHandleEffect.bind(null, create, ref),
	        deps
	      );
	    },
	    useLayoutEffect: function (create, deps) {
	      return mountEffectImpl(4194308, 4, create, deps);
	    },
	    useInsertionEffect: function (create, deps) {
	      mountEffectImpl(4, 2, create, deps);
	    },
	    useMemo: function (nextCreate, deps) {
	      var hook = mountWorkInProgressHook();
	      deps = void 0 === deps ? null : deps;
	      var nextValue = nextCreate();
	      if (shouldDoubleInvokeUserFnsInHooksDEV) {
	        setIsStrictModeForDevtools(!0);
	        try {
	          nextCreate();
	        } finally {
	          setIsStrictModeForDevtools(!1);
	        }
	      }
	      hook.memoizedState = [nextValue, deps];
	      return nextValue;
	    },
	    useReducer: function (reducer, initialArg, init) {
	      var hook = mountWorkInProgressHook();
	      if (void 0 !== init) {
	        var initialState = init(initialArg);
	        if (shouldDoubleInvokeUserFnsInHooksDEV) {
	          setIsStrictModeForDevtools(!0);
	          try {
	            init(initialArg);
	          } finally {
	            setIsStrictModeForDevtools(!1);
	          }
	        }
	      } else initialState = initialArg;
	      hook.memoizedState = hook.baseState = initialState;
	      reducer = {
	        pending: null,
	        lanes: 0,
	        dispatch: null,
	        lastRenderedReducer: reducer,
	        lastRenderedState: initialState
	      };
	      hook.queue = reducer;
	      reducer = reducer.dispatch = dispatchReducerAction.bind(
	        null,
	        currentlyRenderingFiber,
	        reducer
	      );
	      return [hook.memoizedState, reducer];
	    },
	    useRef: function (initialValue) {
	      var hook = mountWorkInProgressHook();
	      initialValue = { current: initialValue };
	      return (hook.memoizedState = initialValue);
	    },
	    useState: function (initialState) {
	      initialState = mountStateImpl(initialState);
	      var queue = initialState.queue,
	        dispatch = dispatchSetState.bind(null, currentlyRenderingFiber, queue);
	      queue.dispatch = dispatch;
	      return [initialState.memoizedState, dispatch];
	    },
	    useDebugValue: mountDebugValue,
	    useDeferredValue: function (value, initialValue) {
	      var hook = mountWorkInProgressHook();
	      return mountDeferredValueImpl(hook, value, initialValue);
	    },
	    useTransition: function () {
	      var stateHook = mountStateImpl(!1);
	      stateHook = startTransition.bind(
	        null,
	        currentlyRenderingFiber,
	        stateHook.queue,
	        !0,
	        !1
	      );
	      mountWorkInProgressHook().memoizedState = stateHook;
	      return [!1, stateHook];
	    },
	    useSyncExternalStore: function (subscribe, getSnapshot, getServerSnapshot) {
	      var fiber = currentlyRenderingFiber,
	        hook = mountWorkInProgressHook();
	      if (isHydrating) {
	        if (void 0 === getServerSnapshot)
	          throw Error(formatProdErrorMessage(407));
	        getServerSnapshot = getServerSnapshot();
	      } else {
	        getServerSnapshot = getSnapshot();
	        if (null === workInProgressRoot)
	          throw Error(formatProdErrorMessage(349));
	        0 !== (workInProgressRootRenderLanes & 127) ||
	          pushStoreConsistencyCheck(fiber, getSnapshot, getServerSnapshot);
	      }
	      hook.memoizedState = getServerSnapshot;
	      var inst = { value: getServerSnapshot, getSnapshot: getSnapshot };
	      hook.queue = inst;
	      mountEffect(subscribeToStore.bind(null, fiber, inst, subscribe), [
	        subscribe
	      ]);
	      fiber.flags |= 2048;
	      pushSimpleEffect(
	        9,
	        { destroy: void 0 },
	        updateStoreInstance.bind(
	          null,
	          fiber,
	          inst,
	          getServerSnapshot,
	          getSnapshot
	        ),
	        null
	      );
	      return getServerSnapshot;
	    },
	    useId: function () {
	      var hook = mountWorkInProgressHook(),
	        identifierPrefix = workInProgressRoot.identifierPrefix;
	      if (isHydrating) {
	        var JSCompiler_inline_result = treeContextOverflow;
	        var idWithLeadingBit = treeContextId;
	        JSCompiler_inline_result =
	          (
	            idWithLeadingBit & ~(1 << (32 - clz32(idWithLeadingBit) - 1))
	          ).toString(32) + JSCompiler_inline_result;
	        identifierPrefix =
	          "_" + identifierPrefix + "R_" + JSCompiler_inline_result;
	        JSCompiler_inline_result = localIdCounter++;
	        0 < JSCompiler_inline_result &&
	          (identifierPrefix += "H" + JSCompiler_inline_result.toString(32));
	        identifierPrefix += "_";
	      } else
	        (JSCompiler_inline_result = globalClientIdCounter++),
	          (identifierPrefix =
	            "_" +
	            identifierPrefix +
	            "r_" +
	            JSCompiler_inline_result.toString(32) +
	            "_");
	      return (hook.memoizedState = identifierPrefix);
	    },
	    useHostTransitionStatus: useHostTransitionStatus,
	    useFormState: mountActionState,
	    useActionState: mountActionState,
	    useOptimistic: function (passthrough) {
	      var hook = mountWorkInProgressHook();
	      hook.memoizedState = hook.baseState = passthrough;
	      var queue = {
	        pending: null,
	        lanes: 0,
	        dispatch: null,
	        lastRenderedReducer: null,
	        lastRenderedState: null
	      };
	      hook.queue = queue;
	      hook = dispatchOptimisticSetState.bind(
	        null,
	        currentlyRenderingFiber,
	        !0,
	        queue
	      );
	      queue.dispatch = hook;
	      return [passthrough, hook];
	    },
	    useMemoCache: useMemoCache,
	    useCacheRefresh: function () {
	      return (mountWorkInProgressHook().memoizedState = refreshCache.bind(
	        null,
	        currentlyRenderingFiber
	      ));
	    },
	    useEffectEvent: function (callback) {
	      var hook = mountWorkInProgressHook(),
	        ref = { impl: callback };
	      hook.memoizedState = ref;
	      return function () {
	        if (0 !== (executionContext & 2))
	          throw Error(formatProdErrorMessage(440));
	        return ref.impl.apply(void 0, arguments);
	      };
	    }
	  },
	  HooksDispatcherOnUpdate = {
	    readContext: readContext,
	    use: use,
	    useCallback: updateCallback,
	    useContext: readContext,
	    useEffect: updateEffect,
	    useImperativeHandle: updateImperativeHandle,
	    useInsertionEffect: updateInsertionEffect,
	    useLayoutEffect: updateLayoutEffect,
	    useMemo: updateMemo,
	    useReducer: updateReducer,
	    useRef: updateRef,
	    useState: function () {
	      return updateReducer(basicStateReducer);
	    },
	    useDebugValue: mountDebugValue,
	    useDeferredValue: function (value, initialValue) {
	      var hook = updateWorkInProgressHook();
	      return updateDeferredValueImpl(
	        hook,
	        currentHook.memoizedState,
	        value,
	        initialValue
	      );
	    },
	    useTransition: function () {
	      var booleanOrThenable = updateReducer(basicStateReducer)[0],
	        start = updateWorkInProgressHook().memoizedState;
	      return [
	        "boolean" === typeof booleanOrThenable
	          ? booleanOrThenable
	          : useThenable(booleanOrThenable),
	        start
	      ];
	    },
	    useSyncExternalStore: updateSyncExternalStore,
	    useId: updateId,
	    useHostTransitionStatus: useHostTransitionStatus,
	    useFormState: updateActionState,
	    useActionState: updateActionState,
	    useOptimistic: function (passthrough, reducer) {
	      var hook = updateWorkInProgressHook();
	      return updateOptimisticImpl(hook, currentHook, passthrough, reducer);
	    },
	    useMemoCache: useMemoCache,
	    useCacheRefresh: updateRefresh
	  };
	HooksDispatcherOnUpdate.useEffectEvent = updateEvent;
	var HooksDispatcherOnRerender = {
	  readContext: readContext,
	  use: use,
	  useCallback: updateCallback,
	  useContext: readContext,
	  useEffect: updateEffect,
	  useImperativeHandle: updateImperativeHandle,
	  useInsertionEffect: updateInsertionEffect,
	  useLayoutEffect: updateLayoutEffect,
	  useMemo: updateMemo,
	  useReducer: rerenderReducer,
	  useRef: updateRef,
	  useState: function () {
	    return rerenderReducer(basicStateReducer);
	  },
	  useDebugValue: mountDebugValue,
	  useDeferredValue: function (value, initialValue) {
	    var hook = updateWorkInProgressHook();
	    return null === currentHook
	      ? mountDeferredValueImpl(hook, value, initialValue)
	      : updateDeferredValueImpl(
	          hook,
	          currentHook.memoizedState,
	          value,
	          initialValue
	        );
	  },
	  useTransition: function () {
	    var booleanOrThenable = rerenderReducer(basicStateReducer)[0],
	      start = updateWorkInProgressHook().memoizedState;
	    return [
	      "boolean" === typeof booleanOrThenable
	        ? booleanOrThenable
	        : useThenable(booleanOrThenable),
	      start
	    ];
	  },
	  useSyncExternalStore: updateSyncExternalStore,
	  useId: updateId,
	  useHostTransitionStatus: useHostTransitionStatus,
	  useFormState: rerenderActionState,
	  useActionState: rerenderActionState,
	  useOptimistic: function (passthrough, reducer) {
	    var hook = updateWorkInProgressHook();
	    if (null !== currentHook)
	      return updateOptimisticImpl(hook, currentHook, passthrough, reducer);
	    hook.baseState = passthrough;
	    return [passthrough, hook.queue.dispatch];
	  },
	  useMemoCache: useMemoCache,
	  useCacheRefresh: updateRefresh
	};
	HooksDispatcherOnRerender.useEffectEvent = updateEvent;
	function applyDerivedStateFromProps(
	  workInProgress,
	  ctor,
	  getDerivedStateFromProps,
	  nextProps
	) {
	  ctor = workInProgress.memoizedState;
	  getDerivedStateFromProps = getDerivedStateFromProps(nextProps, ctor);
	  getDerivedStateFromProps =
	    null === getDerivedStateFromProps || void 0 === getDerivedStateFromProps
	      ? ctor
	      : assign({}, ctor, getDerivedStateFromProps);
	  workInProgress.memoizedState = getDerivedStateFromProps;
	  0 === workInProgress.lanes &&
	    (workInProgress.updateQueue.baseState = getDerivedStateFromProps);
	}
	var classComponentUpdater = {
	  enqueueSetState: function (inst, payload, callback) {
	    inst = inst._reactInternals;
	    var lane = requestUpdateLane(),
	      update = createUpdate(lane);
	    update.payload = payload;
	    void 0 !== callback && null !== callback && (update.callback = callback);
	    payload = enqueueUpdate(inst, update, lane);
	    null !== payload &&
	      (scheduleUpdateOnFiber(payload, inst, lane),
	      entangleTransitions(payload, inst, lane));
	  },
	  enqueueReplaceState: function (inst, payload, callback) {
	    inst = inst._reactInternals;
	    var lane = requestUpdateLane(),
	      update = createUpdate(lane);
	    update.tag = 1;
	    update.payload = payload;
	    void 0 !== callback && null !== callback && (update.callback = callback);
	    payload = enqueueUpdate(inst, update, lane);
	    null !== payload &&
	      (scheduleUpdateOnFiber(payload, inst, lane),
	      entangleTransitions(payload, inst, lane));
	  },
	  enqueueForceUpdate: function (inst, callback) {
	    inst = inst._reactInternals;
	    var lane = requestUpdateLane(),
	      update = createUpdate(lane);
	    update.tag = 2;
	    void 0 !== callback && null !== callback && (update.callback = callback);
	    callback = enqueueUpdate(inst, update, lane);
	    null !== callback &&
	      (scheduleUpdateOnFiber(callback, inst, lane),
	      entangleTransitions(callback, inst, lane));
	  }
	};
	function checkShouldComponentUpdate(
	  workInProgress,
	  ctor,
	  oldProps,
	  newProps,
	  oldState,
	  newState,
	  nextContext
	) {
	  workInProgress = workInProgress.stateNode;
	  return "function" === typeof workInProgress.shouldComponentUpdate
	    ? workInProgress.shouldComponentUpdate(newProps, newState, nextContext)
	    : ctor.prototype && ctor.prototype.isPureReactComponent
	      ? !shallowEqual(oldProps, newProps) || !shallowEqual(oldState, newState)
	      : !0;
	}
	function callComponentWillReceiveProps(
	  workInProgress,
	  instance,
	  newProps,
	  nextContext
	) {
	  workInProgress = instance.state;
	  "function" === typeof instance.componentWillReceiveProps &&
	    instance.componentWillReceiveProps(newProps, nextContext);
	  "function" === typeof instance.UNSAFE_componentWillReceiveProps &&
	    instance.UNSAFE_componentWillReceiveProps(newProps, nextContext);
	  instance.state !== workInProgress &&
	    classComponentUpdater.enqueueReplaceState(instance, instance.state, null);
	}
	function resolveClassComponentProps(Component, baseProps) {
	  var newProps = baseProps;
	  if ("ref" in baseProps) {
	    newProps = {};
	    for (var propName in baseProps)
	      "ref" !== propName && (newProps[propName] = baseProps[propName]);
	  }
	  if ((Component = Component.defaultProps)) {
	    newProps === baseProps && (newProps = assign({}, newProps));
	    for (var propName$73 in Component)
	      void 0 === newProps[propName$73] &&
	        (newProps[propName$73] = Component[propName$73]);
	  }
	  return newProps;
	}
	function defaultOnUncaughtError(error) {
	  reportGlobalError(error);
	}
	function defaultOnCaughtError(error) {
	  console.error(error);
	}
	function defaultOnRecoverableError(error) {
	  reportGlobalError(error);
	}
	function logUncaughtError(root, errorInfo) {
	  try {
	    var onUncaughtError = root.onUncaughtError;
	    onUncaughtError(errorInfo.value, { componentStack: errorInfo.stack });
	  } catch (e$74) {
	    setTimeout(function () {
	      throw e$74;
	    });
	  }
	}
	function logCaughtError(root, boundary, errorInfo) {
	  try {
	    var onCaughtError = root.onCaughtError;
	    onCaughtError(errorInfo.value, {
	      componentStack: errorInfo.stack,
	      errorBoundary: 1 === boundary.tag ? boundary.stateNode : null
	    });
	  } catch (e$75) {
	    setTimeout(function () {
	      throw e$75;
	    });
	  }
	}
	function createRootErrorUpdate(root, errorInfo, lane) {
	  lane = createUpdate(lane);
	  lane.tag = 3;
	  lane.payload = { element: null };
	  lane.callback = function () {
	    logUncaughtError(root, errorInfo);
	  };
	  return lane;
	}
	function createClassErrorUpdate(lane) {
	  lane = createUpdate(lane);
	  lane.tag = 3;
	  return lane;
	}
	function initializeClassErrorUpdate(update, root, fiber, errorInfo) {
	  var getDerivedStateFromError = fiber.type.getDerivedStateFromError;
	  if ("function" === typeof getDerivedStateFromError) {
	    var error = errorInfo.value;
	    update.payload = function () {
	      return getDerivedStateFromError(error);
	    };
	    update.callback = function () {
	      logCaughtError(root, fiber, errorInfo);
	    };
	  }
	  var inst = fiber.stateNode;
	  null !== inst &&
	    "function" === typeof inst.componentDidCatch &&
	    (update.callback = function () {
	      logCaughtError(root, fiber, errorInfo);
	      "function" !== typeof getDerivedStateFromError &&
	        (null === legacyErrorBoundariesThatAlreadyFailed
	          ? (legacyErrorBoundariesThatAlreadyFailed = new Set([this]))
	          : legacyErrorBoundariesThatAlreadyFailed.add(this));
	      var stack = errorInfo.stack;
	      this.componentDidCatch(errorInfo.value, {
	        componentStack: null !== stack ? stack : ""
	      });
	    });
	}
	function throwException(
	  root,
	  returnFiber,
	  sourceFiber,
	  value,
	  rootRenderLanes
	) {
	  sourceFiber.flags |= 32768;
	  if (
	    null !== value &&
	    "object" === typeof value &&
	    "function" === typeof value.then
	  ) {
	    returnFiber = sourceFiber.alternate;
	    null !== returnFiber &&
	      propagateParentContextChanges(
	        returnFiber,
	        sourceFiber,
	        rootRenderLanes,
	        !0
	      );
	    sourceFiber = suspenseHandlerStackCursor.current;
	    if (null !== sourceFiber) {
	      switch (sourceFiber.tag) {
	        case 31:
	        case 13:
	          return (
	            null === shellBoundary
	              ? renderDidSuspendDelayIfPossible()
	              : null === sourceFiber.alternate &&
	                0 === workInProgressRootExitStatus &&
	                (workInProgressRootExitStatus = 3),
	            (sourceFiber.flags &= -257),
	            (sourceFiber.flags |= 65536),
	            (sourceFiber.lanes = rootRenderLanes),
	            value === noopSuspenseyCommitThenable
	              ? (sourceFiber.flags |= 16384)
	              : ((returnFiber = sourceFiber.updateQueue),
	                null === returnFiber
	                  ? (sourceFiber.updateQueue = new Set([value]))
	                  : returnFiber.add(value),
	                attachPingListener(root, value, rootRenderLanes)),
	            !1
	          );
	        case 22:
	          return (
	            (sourceFiber.flags |= 65536),
	            value === noopSuspenseyCommitThenable
	              ? (sourceFiber.flags |= 16384)
	              : ((returnFiber = sourceFiber.updateQueue),
	                null === returnFiber
	                  ? ((returnFiber = {
	                      transitions: null,
	                      markerInstances: null,
	                      retryQueue: new Set([value])
	                    }),
	                    (sourceFiber.updateQueue = returnFiber))
	                  : ((sourceFiber = returnFiber.retryQueue),
	                    null === sourceFiber
	                      ? (returnFiber.retryQueue = new Set([value]))
	                      : sourceFiber.add(value)),
	                attachPingListener(root, value, rootRenderLanes)),
	            !1
	          );
	      }
	      throw Error(formatProdErrorMessage(435, sourceFiber.tag));
	    }
	    attachPingListener(root, value, rootRenderLanes);
	    renderDidSuspendDelayIfPossible();
	    return !1;
	  }
	  if (isHydrating)
	    return (
	      (returnFiber = suspenseHandlerStackCursor.current),
	      null !== returnFiber
	        ? (0 === (returnFiber.flags & 65536) && (returnFiber.flags |= 256),
	          (returnFiber.flags |= 65536),
	          (returnFiber.lanes = rootRenderLanes),
	          value !== HydrationMismatchException &&
	            ((root = Error(formatProdErrorMessage(422), { cause: value })),
	            queueHydrationError(createCapturedValueAtFiber(root, sourceFiber))))
	        : (value !== HydrationMismatchException &&
	            ((returnFiber = Error(formatProdErrorMessage(423), {
	              cause: value
	            })),
	            queueHydrationError(
	              createCapturedValueAtFiber(returnFiber, sourceFiber)
	            )),
	          (root = root.current.alternate),
	          (root.flags |= 65536),
	          (rootRenderLanes &= -rootRenderLanes),
	          (root.lanes |= rootRenderLanes),
	          (value = createCapturedValueAtFiber(value, sourceFiber)),
	          (rootRenderLanes = createRootErrorUpdate(
	            root.stateNode,
	            value,
	            rootRenderLanes
	          )),
	          enqueueCapturedUpdate(root, rootRenderLanes),
	          4 !== workInProgressRootExitStatus &&
	            (workInProgressRootExitStatus = 2)),
	      !1
	    );
	  var wrapperError = Error(formatProdErrorMessage(520), { cause: value });
	  wrapperError = createCapturedValueAtFiber(wrapperError, sourceFiber);
	  null === workInProgressRootConcurrentErrors
	    ? (workInProgressRootConcurrentErrors = [wrapperError])
	    : workInProgressRootConcurrentErrors.push(wrapperError);
	  4 !== workInProgressRootExitStatus && (workInProgressRootExitStatus = 2);
	  if (null === returnFiber) return !0;
	  value = createCapturedValueAtFiber(value, sourceFiber);
	  sourceFiber = returnFiber;
	  do {
	    switch (sourceFiber.tag) {
	      case 3:
	        return (
	          (sourceFiber.flags |= 65536),
	          (root = rootRenderLanes & -rootRenderLanes),
	          (sourceFiber.lanes |= root),
	          (root = createRootErrorUpdate(sourceFiber.stateNode, value, root)),
	          enqueueCapturedUpdate(sourceFiber, root),
	          !1
	        );
	      case 1:
	        if (
	          ((returnFiber = sourceFiber.type),
	          (wrapperError = sourceFiber.stateNode),
	          0 === (sourceFiber.flags & 128) &&
	            ("function" === typeof returnFiber.getDerivedStateFromError ||
	              (null !== wrapperError &&
	                "function" === typeof wrapperError.componentDidCatch &&
	                (null === legacyErrorBoundariesThatAlreadyFailed ||
	                  !legacyErrorBoundariesThatAlreadyFailed.has(wrapperError)))))
	        )
	          return (
	            (sourceFiber.flags |= 65536),
	            (rootRenderLanes &= -rootRenderLanes),
	            (sourceFiber.lanes |= rootRenderLanes),
	            (rootRenderLanes = createClassErrorUpdate(rootRenderLanes)),
	            initializeClassErrorUpdate(
	              rootRenderLanes,
	              root,
	              sourceFiber,
	              value
	            ),
	            enqueueCapturedUpdate(sourceFiber, rootRenderLanes),
	            !1
	          );
	    }
	    sourceFiber = sourceFiber.return;
	  } while (null !== sourceFiber);
	  return !1;
	}
	var SelectiveHydrationException = Error(formatProdErrorMessage(461)),
	  didReceiveUpdate = !1;
	function reconcileChildren(current, workInProgress, nextChildren, renderLanes) {
	  workInProgress.child =
	    null === current
	      ? mountChildFibers(workInProgress, null, nextChildren, renderLanes)
	      : reconcileChildFibers(
	          workInProgress,
	          current.child,
	          nextChildren,
	          renderLanes
	        );
	}
	function updateForwardRef(
	  current,
	  workInProgress,
	  Component,
	  nextProps,
	  renderLanes
	) {
	  Component = Component.render;
	  var ref = workInProgress.ref;
	  if ("ref" in nextProps) {
	    var propsWithoutRef = {};
	    for (var key in nextProps)
	      "ref" !== key && (propsWithoutRef[key] = nextProps[key]);
	  } else propsWithoutRef = nextProps;
	  prepareToReadContext(workInProgress);
	  nextProps = renderWithHooks(
	    current,
	    workInProgress,
	    Component,
	    propsWithoutRef,
	    ref,
	    renderLanes
	  );
	  key = checkDidRenderIdHook();
	  if (null !== current && !didReceiveUpdate)
	    return (
	      bailoutHooks(current, workInProgress, renderLanes),
	      bailoutOnAlreadyFinishedWork(current, workInProgress, renderLanes)
	    );
	  isHydrating && key && pushMaterializedTreeId(workInProgress);
	  workInProgress.flags |= 1;
	  reconcileChildren(current, workInProgress, nextProps, renderLanes);
	  return workInProgress.child;
	}
	function updateMemoComponent(
	  current,
	  workInProgress,
	  Component,
	  nextProps,
	  renderLanes
	) {
	  if (null === current) {
	    var type = Component.type;
	    if (
	      "function" === typeof type &&
	      !shouldConstruct(type) &&
	      void 0 === type.defaultProps &&
	      null === Component.compare
	    )
	      return (
	        (workInProgress.tag = 15),
	        (workInProgress.type = type),
	        updateSimpleMemoComponent(
	          current,
	          workInProgress,
	          type,
	          nextProps,
	          renderLanes
	        )
	      );
	    current = createFiberFromTypeAndProps(
	      Component.type,
	      null,
	      nextProps,
	      workInProgress,
	      workInProgress.mode,
	      renderLanes
	    );
	    current.ref = workInProgress.ref;
	    current.return = workInProgress;
	    return (workInProgress.child = current);
	  }
	  type = current.child;
	  if (!checkScheduledUpdateOrContext(current, renderLanes)) {
	    var prevProps = type.memoizedProps;
	    Component = Component.compare;
	    Component = null !== Component ? Component : shallowEqual;
	    if (Component(prevProps, nextProps) && current.ref === workInProgress.ref)
	      return bailoutOnAlreadyFinishedWork(current, workInProgress, renderLanes);
	  }
	  workInProgress.flags |= 1;
	  current = createWorkInProgress(type, nextProps);
	  current.ref = workInProgress.ref;
	  current.return = workInProgress;
	  return (workInProgress.child = current);
	}
	function updateSimpleMemoComponent(
	  current,
	  workInProgress,
	  Component,
	  nextProps,
	  renderLanes
	) {
	  if (null !== current) {
	    var prevProps = current.memoizedProps;
	    if (
	      shallowEqual(prevProps, nextProps) &&
	      current.ref === workInProgress.ref
	    )
	      if (
	        ((didReceiveUpdate = !1),
	        (workInProgress.pendingProps = nextProps = prevProps),
	        checkScheduledUpdateOrContext(current, renderLanes))
	      )
	        0 !== (current.flags & 131072) && (didReceiveUpdate = !0);
	      else
	        return (
	          (workInProgress.lanes = current.lanes),
	          bailoutOnAlreadyFinishedWork(current, workInProgress, renderLanes)
	        );
	  }
	  return updateFunctionComponent(
	    current,
	    workInProgress,
	    Component,
	    nextProps,
	    renderLanes
	  );
	}
	function updateOffscreenComponent(
	  current,
	  workInProgress,
	  renderLanes,
	  nextProps
	) {
	  var nextChildren = nextProps.children,
	    prevState = null !== current ? current.memoizedState : null;
	  null === current &&
	    null === workInProgress.stateNode &&
	    (workInProgress.stateNode = {
	      _visibility: 1,
	      _pendingMarkers: null,
	      _retryCache: null,
	      _transitions: null
	    });
	  if ("hidden" === nextProps.mode) {
	    if (0 !== (workInProgress.flags & 128)) {
	      prevState =
	        null !== prevState ? prevState.baseLanes | renderLanes : renderLanes;
	      if (null !== current) {
	        nextProps = workInProgress.child = current.child;
	        for (nextChildren = 0; null !== nextProps; )
	          (nextChildren =
	            nextChildren | nextProps.lanes | nextProps.childLanes),
	            (nextProps = nextProps.sibling);
	        nextProps = nextChildren & ~prevState;
	      } else (nextProps = 0), (workInProgress.child = null);
	      return deferHiddenOffscreenComponent(
	        current,
	        workInProgress,
	        prevState,
	        renderLanes,
	        nextProps
	      );
	    }
	    if (0 !== (renderLanes & 536870912))
	      (workInProgress.memoizedState = { baseLanes: 0, cachePool: null }),
	        null !== current &&
	          pushTransition(
	            workInProgress,
	            null !== prevState ? prevState.cachePool : null
	          ),
	        null !== prevState
	          ? pushHiddenContext(workInProgress, prevState)
	          : reuseHiddenContextOnStack(),
	        pushOffscreenSuspenseHandler(workInProgress);
	    else
	      return (
	        (nextProps = workInProgress.lanes = 536870912),
	        deferHiddenOffscreenComponent(
	          current,
	          workInProgress,
	          null !== prevState ? prevState.baseLanes | renderLanes : renderLanes,
	          renderLanes,
	          nextProps
	        )
	      );
	  } else
	    null !== prevState
	      ? (pushTransition(workInProgress, prevState.cachePool),
	        pushHiddenContext(workInProgress, prevState),
	        reuseSuspenseHandlerOnStack(workInProgress),
	        (workInProgress.memoizedState = null))
	      : (null !== current && pushTransition(workInProgress, null),
	        reuseHiddenContextOnStack(),
	        reuseSuspenseHandlerOnStack(workInProgress));
	  reconcileChildren(current, workInProgress, nextChildren, renderLanes);
	  return workInProgress.child;
	}
	function bailoutOffscreenComponent(current, workInProgress) {
	  (null !== current && 22 === current.tag) ||
	    null !== workInProgress.stateNode ||
	    (workInProgress.stateNode = {
	      _visibility: 1,
	      _pendingMarkers: null,
	      _retryCache: null,
	      _transitions: null
	    });
	  return workInProgress.sibling;
	}
	function deferHiddenOffscreenComponent(
	  current,
	  workInProgress,
	  nextBaseLanes,
	  renderLanes,
	  remainingChildLanes
	) {
	  var JSCompiler_inline_result = peekCacheFromPool();
	  JSCompiler_inline_result =
	    null === JSCompiler_inline_result
	      ? null
	      : { parent: CacheContext._currentValue, pool: JSCompiler_inline_result };
	  workInProgress.memoizedState = {
	    baseLanes: nextBaseLanes,
	    cachePool: JSCompiler_inline_result
	  };
	  null !== current && pushTransition(workInProgress, null);
	  reuseHiddenContextOnStack();
	  pushOffscreenSuspenseHandler(workInProgress);
	  null !== current &&
	    propagateParentContextChanges(current, workInProgress, renderLanes, !0);
	  workInProgress.childLanes = remainingChildLanes;
	  return null;
	}
	function mountActivityChildren(workInProgress, nextProps) {
	  nextProps = mountWorkInProgressOffscreenFiber(
	    { mode: nextProps.mode, children: nextProps.children },
	    workInProgress.mode
	  );
	  nextProps.ref = workInProgress.ref;
	  workInProgress.child = nextProps;
	  nextProps.return = workInProgress;
	  return nextProps;
	}
	function retryActivityComponentWithoutHydrating(
	  current,
	  workInProgress,
	  renderLanes
	) {
	  reconcileChildFibers(workInProgress, current.child, null, renderLanes);
	  current = mountActivityChildren(workInProgress, workInProgress.pendingProps);
	  current.flags |= 2;
	  popSuspenseHandler(workInProgress);
	  workInProgress.memoizedState = null;
	  return current;
	}
	function updateActivityComponent(current, workInProgress, renderLanes) {
	  var nextProps = workInProgress.pendingProps,
	    didSuspend = 0 !== (workInProgress.flags & 128);
	  workInProgress.flags &= -129;
	  if (null === current) {
	    if (isHydrating) {
	      if ("hidden" === nextProps.mode)
	        return (
	          (current = mountActivityChildren(workInProgress, nextProps)),
	          (workInProgress.lanes = 536870912),
	          bailoutOffscreenComponent(null, current)
	        );
	      pushDehydratedActivitySuspenseHandler(workInProgress);
	      (current = nextHydratableInstance)
	        ? ((current = canHydrateHydrationBoundary(
	            current,
	            rootOrSingletonContext
	          )),
	          (current = null !== current && "&" === current.data ? current : null),
	          null !== current &&
	            ((workInProgress.memoizedState = {
	              dehydrated: current,
	              treeContext:
	                null !== treeContextProvider
	                  ? { id: treeContextId, overflow: treeContextOverflow }
	                  : null,
	              retryLane: 536870912,
	              hydrationErrors: null
	            }),
	            (renderLanes = createFiberFromDehydratedFragment(current)),
	            (renderLanes.return = workInProgress),
	            (workInProgress.child = renderLanes),
	            (hydrationParentFiber = workInProgress),
	            (nextHydratableInstance = null)))
	        : (current = null);
	      if (null === current) throw throwOnHydrationMismatch(workInProgress);
	      workInProgress.lanes = 536870912;
	      return null;
	    }
	    return mountActivityChildren(workInProgress, nextProps);
	  }
	  var prevState = current.memoizedState;
	  if (null !== prevState) {
	    var dehydrated = prevState.dehydrated;
	    pushDehydratedActivitySuspenseHandler(workInProgress);
	    if (didSuspend)
	      if (workInProgress.flags & 256)
	        (workInProgress.flags &= -257),
	          (workInProgress = retryActivityComponentWithoutHydrating(
	            current,
	            workInProgress,
	            renderLanes
	          ));
	      else if (null !== workInProgress.memoizedState)
	        (workInProgress.child = current.child),
	          (workInProgress.flags |= 128),
	          (workInProgress = null);
	      else throw Error(formatProdErrorMessage(558));
	    else if (
	      (didReceiveUpdate ||
	        propagateParentContextChanges(current, workInProgress, renderLanes, !1),
	      (didSuspend = 0 !== (renderLanes & current.childLanes)),
	      didReceiveUpdate || didSuspend)
	    ) {
	      nextProps = workInProgressRoot;
	      if (
	        null !== nextProps &&
	        ((dehydrated = getBumpedLaneForHydration(nextProps, renderLanes)),
	        0 !== dehydrated && dehydrated !== prevState.retryLane)
	      )
	        throw (
	          ((prevState.retryLane = dehydrated),
	          enqueueConcurrentRenderForLane(current, dehydrated),
	          scheduleUpdateOnFiber(nextProps, current, dehydrated),
	          SelectiveHydrationException)
	        );
	      renderDidSuspendDelayIfPossible();
	      workInProgress = retryActivityComponentWithoutHydrating(
	        current,
	        workInProgress,
	        renderLanes
	      );
	    } else
	      (current = prevState.treeContext),
	        (nextHydratableInstance = getNextHydratable(dehydrated.nextSibling)),
	        (hydrationParentFiber = workInProgress),
	        (isHydrating = !0),
	        (hydrationErrors = null),
	        (rootOrSingletonContext = !1),
	        null !== current &&
	          restoreSuspendedTreeContext(workInProgress, current),
	        (workInProgress = mountActivityChildren(workInProgress, nextProps)),
	        (workInProgress.flags |= 4096);
	    return workInProgress;
	  }
	  current = createWorkInProgress(current.child, {
	    mode: nextProps.mode,
	    children: nextProps.children
	  });
	  current.ref = workInProgress.ref;
	  workInProgress.child = current;
	  current.return = workInProgress;
	  return current;
	}
	function markRef(current, workInProgress) {
	  var ref = workInProgress.ref;
	  if (null === ref)
	    null !== current &&
	      null !== current.ref &&
	      (workInProgress.flags |= 4194816);
	  else {
	    if ("function" !== typeof ref && "object" !== typeof ref)
	      throw Error(formatProdErrorMessage(284));
	    if (null === current || current.ref !== ref)
	      workInProgress.flags |= 4194816;
	  }
	}
	function updateFunctionComponent(
	  current,
	  workInProgress,
	  Component,
	  nextProps,
	  renderLanes
	) {
	  prepareToReadContext(workInProgress);
	  Component = renderWithHooks(
	    current,
	    workInProgress,
	    Component,
	    nextProps,
	    void 0,
	    renderLanes
	  );
	  nextProps = checkDidRenderIdHook();
	  if (null !== current && !didReceiveUpdate)
	    return (
	      bailoutHooks(current, workInProgress, renderLanes),
	      bailoutOnAlreadyFinishedWork(current, workInProgress, renderLanes)
	    );
	  isHydrating && nextProps && pushMaterializedTreeId(workInProgress);
	  workInProgress.flags |= 1;
	  reconcileChildren(current, workInProgress, Component, renderLanes);
	  return workInProgress.child;
	}
	function replayFunctionComponent(
	  current,
	  workInProgress,
	  nextProps,
	  Component,
	  secondArg,
	  renderLanes
	) {
	  prepareToReadContext(workInProgress);
	  workInProgress.updateQueue = null;
	  nextProps = renderWithHooksAgain(
	    workInProgress,
	    Component,
	    nextProps,
	    secondArg
	  );
	  finishRenderingHooks(current);
	  Component = checkDidRenderIdHook();
	  if (null !== current && !didReceiveUpdate)
	    return (
	      bailoutHooks(current, workInProgress, renderLanes),
	      bailoutOnAlreadyFinishedWork(current, workInProgress, renderLanes)
	    );
	  isHydrating && Component && pushMaterializedTreeId(workInProgress);
	  workInProgress.flags |= 1;
	  reconcileChildren(current, workInProgress, nextProps, renderLanes);
	  return workInProgress.child;
	}
	function updateClassComponent(
	  current,
	  workInProgress,
	  Component,
	  nextProps,
	  renderLanes
	) {
	  prepareToReadContext(workInProgress);
	  if (null === workInProgress.stateNode) {
	    var context = emptyContextObject,
	      contextType = Component.contextType;
	    "object" === typeof contextType &&
	      null !== contextType &&
	      (context = readContext(contextType));
	    context = new Component(nextProps, context);
	    workInProgress.memoizedState =
	      null !== context.state && void 0 !== context.state ? context.state : null;
	    context.updater = classComponentUpdater;
	    workInProgress.stateNode = context;
	    context._reactInternals = workInProgress;
	    context = workInProgress.stateNode;
	    context.props = nextProps;
	    context.state = workInProgress.memoizedState;
	    context.refs = {};
	    initializeUpdateQueue(workInProgress);
	    contextType = Component.contextType;
	    context.context =
	      "object" === typeof contextType && null !== contextType
	        ? readContext(contextType)
	        : emptyContextObject;
	    context.state = workInProgress.memoizedState;
	    contextType = Component.getDerivedStateFromProps;
	    "function" === typeof contextType &&
	      (applyDerivedStateFromProps(
	        workInProgress,
	        Component,
	        contextType,
	        nextProps
	      ),
	      (context.state = workInProgress.memoizedState));
	    "function" === typeof Component.getDerivedStateFromProps ||
	      "function" === typeof context.getSnapshotBeforeUpdate ||
	      ("function" !== typeof context.UNSAFE_componentWillMount &&
	        "function" !== typeof context.componentWillMount) ||
	      ((contextType = context.state),
	      "function" === typeof context.componentWillMount &&
	        context.componentWillMount(),
	      "function" === typeof context.UNSAFE_componentWillMount &&
	        context.UNSAFE_componentWillMount(),
	      contextType !== context.state &&
	        classComponentUpdater.enqueueReplaceState(context, context.state, null),
	      processUpdateQueue(workInProgress, nextProps, context, renderLanes),
	      suspendIfUpdateReadFromEntangledAsyncAction(),
	      (context.state = workInProgress.memoizedState));
	    "function" === typeof context.componentDidMount &&
	      (workInProgress.flags |= 4194308);
	    nextProps = !0;
	  } else if (null === current) {
	    context = workInProgress.stateNode;
	    var unresolvedOldProps = workInProgress.memoizedProps,
	      oldProps = resolveClassComponentProps(Component, unresolvedOldProps);
	    context.props = oldProps;
	    var oldContext = context.context,
	      contextType$jscomp$0 = Component.contextType;
	    contextType = emptyContextObject;
	    "object" === typeof contextType$jscomp$0 &&
	      null !== contextType$jscomp$0 &&
	      (contextType = readContext(contextType$jscomp$0));
	    var getDerivedStateFromProps = Component.getDerivedStateFromProps;
	    contextType$jscomp$0 =
	      "function" === typeof getDerivedStateFromProps ||
	      "function" === typeof context.getSnapshotBeforeUpdate;
	    unresolvedOldProps = workInProgress.pendingProps !== unresolvedOldProps;
	    contextType$jscomp$0 ||
	      ("function" !== typeof context.UNSAFE_componentWillReceiveProps &&
	        "function" !== typeof context.componentWillReceiveProps) ||
	      ((unresolvedOldProps || oldContext !== contextType) &&
	        callComponentWillReceiveProps(
	          workInProgress,
	          context,
	          nextProps,
	          contextType
	        ));
	    hasForceUpdate = !1;
	    var oldState = workInProgress.memoizedState;
	    context.state = oldState;
	    processUpdateQueue(workInProgress, nextProps, context, renderLanes);
	    suspendIfUpdateReadFromEntangledAsyncAction();
	    oldContext = workInProgress.memoizedState;
	    unresolvedOldProps || oldState !== oldContext || hasForceUpdate
	      ? ("function" === typeof getDerivedStateFromProps &&
	          (applyDerivedStateFromProps(
	            workInProgress,
	            Component,
	            getDerivedStateFromProps,
	            nextProps
	          ),
	          (oldContext = workInProgress.memoizedState)),
	        (oldProps =
	          hasForceUpdate ||
	          checkShouldComponentUpdate(
	            workInProgress,
	            Component,
	            oldProps,
	            nextProps,
	            oldState,
	            oldContext,
	            contextType
	          ))
	          ? (contextType$jscomp$0 ||
	              ("function" !== typeof context.UNSAFE_componentWillMount &&
	                "function" !== typeof context.componentWillMount) ||
	              ("function" === typeof context.componentWillMount &&
	                context.componentWillMount(),
	              "function" === typeof context.UNSAFE_componentWillMount &&
	                context.UNSAFE_componentWillMount()),
	            "function" === typeof context.componentDidMount &&
	              (workInProgress.flags |= 4194308))
	          : ("function" === typeof context.componentDidMount &&
	              (workInProgress.flags |= 4194308),
	            (workInProgress.memoizedProps = nextProps),
	            (workInProgress.memoizedState = oldContext)),
	        (context.props = nextProps),
	        (context.state = oldContext),
	        (context.context = contextType),
	        (nextProps = oldProps))
	      : ("function" === typeof context.componentDidMount &&
	          (workInProgress.flags |= 4194308),
	        (nextProps = !1));
	  } else {
	    context = workInProgress.stateNode;
	    cloneUpdateQueue(current, workInProgress);
	    contextType = workInProgress.memoizedProps;
	    contextType$jscomp$0 = resolveClassComponentProps(Component, contextType);
	    context.props = contextType$jscomp$0;
	    getDerivedStateFromProps = workInProgress.pendingProps;
	    oldState = context.context;
	    oldContext = Component.contextType;
	    oldProps = emptyContextObject;
	    "object" === typeof oldContext &&
	      null !== oldContext &&
	      (oldProps = readContext(oldContext));
	    unresolvedOldProps = Component.getDerivedStateFromProps;
	    (oldContext =
	      "function" === typeof unresolvedOldProps ||
	      "function" === typeof context.getSnapshotBeforeUpdate) ||
	      ("function" !== typeof context.UNSAFE_componentWillReceiveProps &&
	        "function" !== typeof context.componentWillReceiveProps) ||
	      ((contextType !== getDerivedStateFromProps || oldState !== oldProps) &&
	        callComponentWillReceiveProps(
	          workInProgress,
	          context,
	          nextProps,
	          oldProps
	        ));
	    hasForceUpdate = !1;
	    oldState = workInProgress.memoizedState;
	    context.state = oldState;
	    processUpdateQueue(workInProgress, nextProps, context, renderLanes);
	    suspendIfUpdateReadFromEntangledAsyncAction();
	    var newState = workInProgress.memoizedState;
	    contextType !== getDerivedStateFromProps ||
	    oldState !== newState ||
	    hasForceUpdate ||
	    (null !== current &&
	      null !== current.dependencies &&
	      checkIfContextChanged(current.dependencies))
	      ? ("function" === typeof unresolvedOldProps &&
	          (applyDerivedStateFromProps(
	            workInProgress,
	            Component,
	            unresolvedOldProps,
	            nextProps
	          ),
	          (newState = workInProgress.memoizedState)),
	        (contextType$jscomp$0 =
	          hasForceUpdate ||
	          checkShouldComponentUpdate(
	            workInProgress,
	            Component,
	            contextType$jscomp$0,
	            nextProps,
	            oldState,
	            newState,
	            oldProps
	          ) ||
	          (null !== current &&
	            null !== current.dependencies &&
	            checkIfContextChanged(current.dependencies)))
	          ? (oldContext ||
	              ("function" !== typeof context.UNSAFE_componentWillUpdate &&
	                "function" !== typeof context.componentWillUpdate) ||
	              ("function" === typeof context.componentWillUpdate &&
	                context.componentWillUpdate(nextProps, newState, oldProps),
	              "function" === typeof context.UNSAFE_componentWillUpdate &&
	                context.UNSAFE_componentWillUpdate(
	                  nextProps,
	                  newState,
	                  oldProps
	                )),
	            "function" === typeof context.componentDidUpdate &&
	              (workInProgress.flags |= 4),
	            "function" === typeof context.getSnapshotBeforeUpdate &&
	              (workInProgress.flags |= 1024))
	          : ("function" !== typeof context.componentDidUpdate ||
	              (contextType === current.memoizedProps &&
	                oldState === current.memoizedState) ||
	              (workInProgress.flags |= 4),
	            "function" !== typeof context.getSnapshotBeforeUpdate ||
	              (contextType === current.memoizedProps &&
	                oldState === current.memoizedState) ||
	              (workInProgress.flags |= 1024),
	            (workInProgress.memoizedProps = nextProps),
	            (workInProgress.memoizedState = newState)),
	        (context.props = nextProps),
	        (context.state = newState),
	        (context.context = oldProps),
	        (nextProps = contextType$jscomp$0))
	      : ("function" !== typeof context.componentDidUpdate ||
	          (contextType === current.memoizedProps &&
	            oldState === current.memoizedState) ||
	          (workInProgress.flags |= 4),
	        "function" !== typeof context.getSnapshotBeforeUpdate ||
	          (contextType === current.memoizedProps &&
	            oldState === current.memoizedState) ||
	          (workInProgress.flags |= 1024),
	        (nextProps = !1));
	  }
	  context = nextProps;
	  markRef(current, workInProgress);
	  nextProps = 0 !== (workInProgress.flags & 128);
	  context || nextProps
	    ? ((context = workInProgress.stateNode),
	      (Component =
	        nextProps && "function" !== typeof Component.getDerivedStateFromError
	          ? null
	          : context.render()),
	      (workInProgress.flags |= 1),
	      null !== current && nextProps
	        ? ((workInProgress.child = reconcileChildFibers(
	            workInProgress,
	            current.child,
	            null,
	            renderLanes
	          )),
	          (workInProgress.child = reconcileChildFibers(
	            workInProgress,
	            null,
	            Component,
	            renderLanes
	          )))
	        : reconcileChildren(current, workInProgress, Component, renderLanes),
	      (workInProgress.memoizedState = context.state),
	      (current = workInProgress.child))
	    : (current = bailoutOnAlreadyFinishedWork(
	        current,
	        workInProgress,
	        renderLanes
	      ));
	  return current;
	}
	function mountHostRootWithoutHydrating(
	  current,
	  workInProgress,
	  nextChildren,
	  renderLanes
	) {
	  resetHydrationState();
	  workInProgress.flags |= 256;
	  reconcileChildren(current, workInProgress, nextChildren, renderLanes);
	  return workInProgress.child;
	}
	var SUSPENDED_MARKER = {
	  dehydrated: null,
	  treeContext: null,
	  retryLane: 0,
	  hydrationErrors: null
	};
	function mountSuspenseOffscreenState(renderLanes) {
	  return { baseLanes: renderLanes, cachePool: getSuspendedCache() };
	}
	function getRemainingWorkInPrimaryTree(
	  current,
	  primaryTreeDidDefer,
	  renderLanes
	) {
	  current = null !== current ? current.childLanes & ~renderLanes : 0;
	  primaryTreeDidDefer && (current |= workInProgressDeferredLane);
	  return current;
	}
	function updateSuspenseComponent(current, workInProgress, renderLanes) {
	  var nextProps = workInProgress.pendingProps,
	    showFallback = !1,
	    didSuspend = 0 !== (workInProgress.flags & 128),
	    JSCompiler_temp;
	  (JSCompiler_temp = didSuspend) ||
	    (JSCompiler_temp =
	      null !== current && null === current.memoizedState
	        ? !1
	        : 0 !== (suspenseStackCursor.current & 2));
	  JSCompiler_temp && ((showFallback = !0), (workInProgress.flags &= -129));
	  JSCompiler_temp = 0 !== (workInProgress.flags & 32);
	  workInProgress.flags &= -33;
	  if (null === current) {
	    if (isHydrating) {
	      showFallback
	        ? pushPrimaryTreeSuspenseHandler(workInProgress)
	        : reuseSuspenseHandlerOnStack(workInProgress);
	      (current = nextHydratableInstance)
	        ? ((current = canHydrateHydrationBoundary(
	            current,
	            rootOrSingletonContext
	          )),
	          (current = null !== current && "&" !== current.data ? current : null),
	          null !== current &&
	            ((workInProgress.memoizedState = {
	              dehydrated: current,
	              treeContext:
	                null !== treeContextProvider
	                  ? { id: treeContextId, overflow: treeContextOverflow }
	                  : null,
	              retryLane: 536870912,
	              hydrationErrors: null
	            }),
	            (renderLanes = createFiberFromDehydratedFragment(current)),
	            (renderLanes.return = workInProgress),
	            (workInProgress.child = renderLanes),
	            (hydrationParentFiber = workInProgress),
	            (nextHydratableInstance = null)))
	        : (current = null);
	      if (null === current) throw throwOnHydrationMismatch(workInProgress);
	      isSuspenseInstanceFallback(current)
	        ? (workInProgress.lanes = 32)
	        : (workInProgress.lanes = 536870912);
	      return null;
	    }
	    var nextPrimaryChildren = nextProps.children;
	    nextProps = nextProps.fallback;
	    if (showFallback)
	      return (
	        reuseSuspenseHandlerOnStack(workInProgress),
	        (showFallback = workInProgress.mode),
	        (nextPrimaryChildren = mountWorkInProgressOffscreenFiber(
	          { mode: "hidden", children: nextPrimaryChildren },
	          showFallback
	        )),
	        (nextProps = createFiberFromFragment(
	          nextProps,
	          showFallback,
	          renderLanes,
	          null
	        )),
	        (nextPrimaryChildren.return = workInProgress),
	        (nextProps.return = workInProgress),
	        (nextPrimaryChildren.sibling = nextProps),
	        (workInProgress.child = nextPrimaryChildren),
	        (nextProps = workInProgress.child),
	        (nextProps.memoizedState = mountSuspenseOffscreenState(renderLanes)),
	        (nextProps.childLanes = getRemainingWorkInPrimaryTree(
	          current,
	          JSCompiler_temp,
	          renderLanes
	        )),
	        (workInProgress.memoizedState = SUSPENDED_MARKER),
	        bailoutOffscreenComponent(null, nextProps)
	      );
	    pushPrimaryTreeSuspenseHandler(workInProgress);
	    return mountSuspensePrimaryChildren(workInProgress, nextPrimaryChildren);
	  }
	  var prevState = current.memoizedState;
	  if (
	    null !== prevState &&
	    ((nextPrimaryChildren = prevState.dehydrated), null !== nextPrimaryChildren)
	  ) {
	    if (didSuspend)
	      workInProgress.flags & 256
	        ? (pushPrimaryTreeSuspenseHandler(workInProgress),
	          (workInProgress.flags &= -257),
	          (workInProgress = retrySuspenseComponentWithoutHydrating(
	            current,
	            workInProgress,
	            renderLanes
	          )))
	        : null !== workInProgress.memoizedState
	          ? (reuseSuspenseHandlerOnStack(workInProgress),
	            (workInProgress.child = current.child),
	            (workInProgress.flags |= 128),
	            (workInProgress = null))
	          : (reuseSuspenseHandlerOnStack(workInProgress),
	            (nextPrimaryChildren = nextProps.fallback),
	            (showFallback = workInProgress.mode),
	            (nextProps = mountWorkInProgressOffscreenFiber(
	              { mode: "visible", children: nextProps.children },
	              showFallback
	            )),
	            (nextPrimaryChildren = createFiberFromFragment(
	              nextPrimaryChildren,
	              showFallback,
	              renderLanes,
	              null
	            )),
	            (nextPrimaryChildren.flags |= 2),
	            (nextProps.return = workInProgress),
	            (nextPrimaryChildren.return = workInProgress),
	            (nextProps.sibling = nextPrimaryChildren),
	            (workInProgress.child = nextProps),
	            reconcileChildFibers(
	              workInProgress,
	              current.child,
	              null,
	              renderLanes
	            ),
	            (nextProps = workInProgress.child),
	            (nextProps.memoizedState =
	              mountSuspenseOffscreenState(renderLanes)),
	            (nextProps.childLanes = getRemainingWorkInPrimaryTree(
	              current,
	              JSCompiler_temp,
	              renderLanes
	            )),
	            (workInProgress.memoizedState = SUSPENDED_MARKER),
	            (workInProgress = bailoutOffscreenComponent(null, nextProps)));
	    else if (
	      (pushPrimaryTreeSuspenseHandler(workInProgress),
	      isSuspenseInstanceFallback(nextPrimaryChildren))
	    ) {
	      JSCompiler_temp =
	        nextPrimaryChildren.nextSibling &&
	        nextPrimaryChildren.nextSibling.dataset;
	      if (JSCompiler_temp) var digest = JSCompiler_temp.dgst;
	      JSCompiler_temp = digest;
	      nextProps = Error(formatProdErrorMessage(419));
	      nextProps.stack = "";
	      nextProps.digest = JSCompiler_temp;
	      queueHydrationError({ value: nextProps, source: null, stack: null });
	      workInProgress = retrySuspenseComponentWithoutHydrating(
	        current,
	        workInProgress,
	        renderLanes
	      );
	    } else if (
	      (didReceiveUpdate ||
	        propagateParentContextChanges(current, workInProgress, renderLanes, !1),
	      (JSCompiler_temp = 0 !== (renderLanes & current.childLanes)),
	      didReceiveUpdate || JSCompiler_temp)
	    ) {
	      JSCompiler_temp = workInProgressRoot;
	      if (
	        null !== JSCompiler_temp &&
	        ((nextProps = getBumpedLaneForHydration(JSCompiler_temp, renderLanes)),
	        0 !== nextProps && nextProps !== prevState.retryLane)
	      )
	        throw (
	          ((prevState.retryLane = nextProps),
	          enqueueConcurrentRenderForLane(current, nextProps),
	          scheduleUpdateOnFiber(JSCompiler_temp, current, nextProps),
	          SelectiveHydrationException)
	        );
	      isSuspenseInstancePending(nextPrimaryChildren) ||
	        renderDidSuspendDelayIfPossible();
	      workInProgress = retrySuspenseComponentWithoutHydrating(
	        current,
	        workInProgress,
	        renderLanes
	      );
	    } else
	      isSuspenseInstancePending(nextPrimaryChildren)
	        ? ((workInProgress.flags |= 192),
	          (workInProgress.child = current.child),
	          (workInProgress = null))
	        : ((current = prevState.treeContext),
	          (nextHydratableInstance = getNextHydratable(
	            nextPrimaryChildren.nextSibling
	          )),
	          (hydrationParentFiber = workInProgress),
	          (isHydrating = !0),
	          (hydrationErrors = null),
	          (rootOrSingletonContext = !1),
	          null !== current &&
	            restoreSuspendedTreeContext(workInProgress, current),
	          (workInProgress = mountSuspensePrimaryChildren(
	            workInProgress,
	            nextProps.children
	          )),
	          (workInProgress.flags |= 4096));
	    return workInProgress;
	  }
	  if (showFallback)
	    return (
	      reuseSuspenseHandlerOnStack(workInProgress),
	      (nextPrimaryChildren = nextProps.fallback),
	      (showFallback = workInProgress.mode),
	      (prevState = current.child),
	      (digest = prevState.sibling),
	      (nextProps = createWorkInProgress(prevState, {
	        mode: "hidden",
	        children: nextProps.children
	      })),
	      (nextProps.subtreeFlags = prevState.subtreeFlags & 65011712),
	      null !== digest
	        ? (nextPrimaryChildren = createWorkInProgress(
	            digest,
	            nextPrimaryChildren
	          ))
	        : ((nextPrimaryChildren = createFiberFromFragment(
	            nextPrimaryChildren,
	            showFallback,
	            renderLanes,
	            null
	          )),
	          (nextPrimaryChildren.flags |= 2)),
	      (nextPrimaryChildren.return = workInProgress),
	      (nextProps.return = workInProgress),
	      (nextProps.sibling = nextPrimaryChildren),
	      (workInProgress.child = nextProps),
	      bailoutOffscreenComponent(null, nextProps),
	      (nextProps = workInProgress.child),
	      (nextPrimaryChildren = current.child.memoizedState),
	      null === nextPrimaryChildren
	        ? (nextPrimaryChildren = mountSuspenseOffscreenState(renderLanes))
	        : ((showFallback = nextPrimaryChildren.cachePool),
	          null !== showFallback
	            ? ((prevState = CacheContext._currentValue),
	              (showFallback =
	                showFallback.parent !== prevState
	                  ? { parent: prevState, pool: prevState }
	                  : showFallback))
	            : (showFallback = getSuspendedCache()),
	          (nextPrimaryChildren = {
	            baseLanes: nextPrimaryChildren.baseLanes | renderLanes,
	            cachePool: showFallback
	          })),
	      (nextProps.memoizedState = nextPrimaryChildren),
	      (nextProps.childLanes = getRemainingWorkInPrimaryTree(
	        current,
	        JSCompiler_temp,
	        renderLanes
	      )),
	      (workInProgress.memoizedState = SUSPENDED_MARKER),
	      bailoutOffscreenComponent(current.child, nextProps)
	    );
	  pushPrimaryTreeSuspenseHandler(workInProgress);
	  renderLanes = current.child;
	  current = renderLanes.sibling;
	  renderLanes = createWorkInProgress(renderLanes, {
	    mode: "visible",
	    children: nextProps.children
	  });
	  renderLanes.return = workInProgress;
	  renderLanes.sibling = null;
	  null !== current &&
	    ((JSCompiler_temp = workInProgress.deletions),
	    null === JSCompiler_temp
	      ? ((workInProgress.deletions = [current]), (workInProgress.flags |= 16))
	      : JSCompiler_temp.push(current));
	  workInProgress.child = renderLanes;
	  workInProgress.memoizedState = null;
	  return renderLanes;
	}
	function mountSuspensePrimaryChildren(workInProgress, primaryChildren) {
	  primaryChildren = mountWorkInProgressOffscreenFiber(
	    { mode: "visible", children: primaryChildren },
	    workInProgress.mode
	  );
	  primaryChildren.return = workInProgress;
	  return (workInProgress.child = primaryChildren);
	}
	function mountWorkInProgressOffscreenFiber(offscreenProps, mode) {
	  offscreenProps = createFiberImplClass(22, offscreenProps, null, mode);
	  offscreenProps.lanes = 0;
	  return offscreenProps;
	}
	function retrySuspenseComponentWithoutHydrating(
	  current,
	  workInProgress,
	  renderLanes
	) {
	  reconcileChildFibers(workInProgress, current.child, null, renderLanes);
	  current = mountSuspensePrimaryChildren(
	    workInProgress,
	    workInProgress.pendingProps.children
	  );
	  current.flags |= 2;
	  workInProgress.memoizedState = null;
	  return current;
	}
	function scheduleSuspenseWorkOnFiber(fiber, renderLanes, propagationRoot) {
	  fiber.lanes |= renderLanes;
	  var alternate = fiber.alternate;
	  null !== alternate && (alternate.lanes |= renderLanes);
	  scheduleContextWorkOnParentPath(fiber.return, renderLanes, propagationRoot);
	}
	function initSuspenseListRenderState(
	  workInProgress,
	  isBackwards,
	  tail,
	  lastContentRow,
	  tailMode,
	  treeForkCount
	) {
	  var renderState = workInProgress.memoizedState;
	  null === renderState
	    ? (workInProgress.memoizedState = {
	        isBackwards: isBackwards,
	        rendering: null,
	        renderingStartTime: 0,
	        last: lastContentRow,
	        tail: tail,
	        tailMode: tailMode,
	        treeForkCount: treeForkCount
	      })
	    : ((renderState.isBackwards = isBackwards),
	      (renderState.rendering = null),
	      (renderState.renderingStartTime = 0),
	      (renderState.last = lastContentRow),
	      (renderState.tail = tail),
	      (renderState.tailMode = tailMode),
	      (renderState.treeForkCount = treeForkCount));
	}
	function updateSuspenseListComponent(current, workInProgress, renderLanes) {
	  var nextProps = workInProgress.pendingProps,
	    revealOrder = nextProps.revealOrder,
	    tailMode = nextProps.tail;
	  nextProps = nextProps.children;
	  var suspenseContext = suspenseStackCursor.current,
	    shouldForceFallback = 0 !== (suspenseContext & 2);
	  shouldForceFallback
	    ? ((suspenseContext = (suspenseContext & 1) | 2),
	      (workInProgress.flags |= 128))
	    : (suspenseContext &= 1);
	  push(suspenseStackCursor, suspenseContext);
	  reconcileChildren(current, workInProgress, nextProps, renderLanes);
	  nextProps = isHydrating ? treeForkCount : 0;
	  if (!shouldForceFallback && null !== current && 0 !== (current.flags & 128))
	    a: for (current = workInProgress.child; null !== current; ) {
	      if (13 === current.tag)
	        null !== current.memoizedState &&
	          scheduleSuspenseWorkOnFiber(current, renderLanes, workInProgress);
	      else if (19 === current.tag)
	        scheduleSuspenseWorkOnFiber(current, renderLanes, workInProgress);
	      else if (null !== current.child) {
	        current.child.return = current;
	        current = current.child;
	        continue;
	      }
	      if (current === workInProgress) break a;
	      for (; null === current.sibling; ) {
	        if (null === current.return || current.return === workInProgress)
	          break a;
	        current = current.return;
	      }
	      current.sibling.return = current.return;
	      current = current.sibling;
	    }
	  switch (revealOrder) {
	    case "forwards":
	      renderLanes = workInProgress.child;
	      for (revealOrder = null; null !== renderLanes; )
	        (current = renderLanes.alternate),
	          null !== current &&
	            null === findFirstSuspended(current) &&
	            (revealOrder = renderLanes),
	          (renderLanes = renderLanes.sibling);
	      renderLanes = revealOrder;
	      null === renderLanes
	        ? ((revealOrder = workInProgress.child), (workInProgress.child = null))
	        : ((revealOrder = renderLanes.sibling), (renderLanes.sibling = null));
	      initSuspenseListRenderState(
	        workInProgress,
	        !1,
	        revealOrder,
	        renderLanes,
	        tailMode,
	        nextProps
	      );
	      break;
	    case "backwards":
	    case "unstable_legacy-backwards":
	      renderLanes = null;
	      revealOrder = workInProgress.child;
	      for (workInProgress.child = null; null !== revealOrder; ) {
	        current = revealOrder.alternate;
	        if (null !== current && null === findFirstSuspended(current)) {
	          workInProgress.child = revealOrder;
	          break;
	        }
	        current = revealOrder.sibling;
	        revealOrder.sibling = renderLanes;
	        renderLanes = revealOrder;
	        revealOrder = current;
	      }
	      initSuspenseListRenderState(
	        workInProgress,
	        !0,
	        renderLanes,
	        null,
	        tailMode,
	        nextProps
	      );
	      break;
	    case "together":
	      initSuspenseListRenderState(
	        workInProgress,
	        !1,
	        null,
	        null,
	        void 0,
	        nextProps
	      );
	      break;
	    default:
	      workInProgress.memoizedState = null;
	  }
	  return workInProgress.child;
	}
	function bailoutOnAlreadyFinishedWork(current, workInProgress, renderLanes) {
	  null !== current && (workInProgress.dependencies = current.dependencies);
	  workInProgressRootSkippedLanes |= workInProgress.lanes;
	  if (0 === (renderLanes & workInProgress.childLanes))
	    if (null !== current) {
	      if (
	        (propagateParentContextChanges(
	          current,
	          workInProgress,
	          renderLanes,
	          !1
	        ),
	        0 === (renderLanes & workInProgress.childLanes))
	      )
	        return null;
	    } else return null;
	  if (null !== current && workInProgress.child !== current.child)
	    throw Error(formatProdErrorMessage(153));
	  if (null !== workInProgress.child) {
	    current = workInProgress.child;
	    renderLanes = createWorkInProgress(current, current.pendingProps);
	    workInProgress.child = renderLanes;
	    for (renderLanes.return = workInProgress; null !== current.sibling; )
	      (current = current.sibling),
	        (renderLanes = renderLanes.sibling =
	          createWorkInProgress(current, current.pendingProps)),
	        (renderLanes.return = workInProgress);
	    renderLanes.sibling = null;
	  }
	  return workInProgress.child;
	}
	function checkScheduledUpdateOrContext(current, renderLanes) {
	  if (0 !== (current.lanes & renderLanes)) return !0;
	  current = current.dependencies;
	  return null !== current && checkIfContextChanged(current) ? !0 : !1;
	}
	function attemptEarlyBailoutIfNoScheduledUpdate(
	  current,
	  workInProgress,
	  renderLanes
	) {
	  switch (workInProgress.tag) {
	    case 3:
	      pushHostContainer(workInProgress, workInProgress.stateNode.containerInfo);
	      pushProvider(workInProgress, CacheContext, current.memoizedState.cache);
	      resetHydrationState();
	      break;
	    case 27:
	    case 5:
	      pushHostContext(workInProgress);
	      break;
	    case 4:
	      pushHostContainer(workInProgress, workInProgress.stateNode.containerInfo);
	      break;
	    case 10:
	      pushProvider(
	        workInProgress,
	        workInProgress.type,
	        workInProgress.memoizedProps.value
	      );
	      break;
	    case 31:
	      if (null !== workInProgress.memoizedState)
	        return (
	          (workInProgress.flags |= 128),
	          pushDehydratedActivitySuspenseHandler(workInProgress),
	          null
	        );
	      break;
	    case 13:
	      var state$102 = workInProgress.memoizedState;
	      if (null !== state$102) {
	        if (null !== state$102.dehydrated)
	          return (
	            pushPrimaryTreeSuspenseHandler(workInProgress),
	            (workInProgress.flags |= 128),
	            null
	          );
	        if (0 !== (renderLanes & workInProgress.child.childLanes))
	          return updateSuspenseComponent(current, workInProgress, renderLanes);
	        pushPrimaryTreeSuspenseHandler(workInProgress);
	        current = bailoutOnAlreadyFinishedWork(
	          current,
	          workInProgress,
	          renderLanes
	        );
	        return null !== current ? current.sibling : null;
	      }
	      pushPrimaryTreeSuspenseHandler(workInProgress);
	      break;
	    case 19:
	      var didSuspendBefore = 0 !== (current.flags & 128);
	      state$102 = 0 !== (renderLanes & workInProgress.childLanes);
	      state$102 ||
	        (propagateParentContextChanges(
	          current,
	          workInProgress,
	          renderLanes,
	          !1
	        ),
	        (state$102 = 0 !== (renderLanes & workInProgress.childLanes)));
	      if (didSuspendBefore) {
	        if (state$102)
	          return updateSuspenseListComponent(
	            current,
	            workInProgress,
	            renderLanes
	          );
	        workInProgress.flags |= 128;
	      }
	      didSuspendBefore = workInProgress.memoizedState;
	      null !== didSuspendBefore &&
	        ((didSuspendBefore.rendering = null),
	        (didSuspendBefore.tail = null),
	        (didSuspendBefore.lastEffect = null));
	      push(suspenseStackCursor, suspenseStackCursor.current);
	      if (state$102) break;
	      else return null;
	    case 22:
	      return (
	        (workInProgress.lanes = 0),
	        updateOffscreenComponent(
	          current,
	          workInProgress,
	          renderLanes,
	          workInProgress.pendingProps
	        )
	      );
	    case 24:
	      pushProvider(workInProgress, CacheContext, current.memoizedState.cache);
	  }
	  return bailoutOnAlreadyFinishedWork(current, workInProgress, renderLanes);
	}
	function beginWork(current, workInProgress, renderLanes) {
	  if (null !== current)
	    if (current.memoizedProps !== workInProgress.pendingProps)
	      didReceiveUpdate = !0;
	    else {
	      if (
	        !checkScheduledUpdateOrContext(current, renderLanes) &&
	        0 === (workInProgress.flags & 128)
	      )
	        return (
	          (didReceiveUpdate = !1),
	          attemptEarlyBailoutIfNoScheduledUpdate(
	            current,
	            workInProgress,
	            renderLanes
	          )
	        );
	      didReceiveUpdate = 0 !== (current.flags & 131072) ? !0 : !1;
	    }
	  else
	    (didReceiveUpdate = !1),
	      isHydrating &&
	        0 !== (workInProgress.flags & 1048576) &&
	        pushTreeId(workInProgress, treeForkCount, workInProgress.index);
	  workInProgress.lanes = 0;
	  switch (workInProgress.tag) {
	    case 16:
	      a: {
	        var props = workInProgress.pendingProps;
	        current = resolveLazy(workInProgress.elementType);
	        workInProgress.type = current;
	        if ("function" === typeof current)
	          shouldConstruct(current)
	            ? ((props = resolveClassComponentProps(current, props)),
	              (workInProgress.tag = 1),
	              (workInProgress = updateClassComponent(
	                null,
	                workInProgress,
	                current,
	                props,
	                renderLanes
	              )))
	            : ((workInProgress.tag = 0),
	              (workInProgress = updateFunctionComponent(
	                null,
	                workInProgress,
	                current,
	                props,
	                renderLanes
	              )));
	        else {
	          if (void 0 !== current && null !== current) {
	            var $$typeof = current.$$typeof;
	            if ($$typeof === REACT_FORWARD_REF_TYPE) {
	              workInProgress.tag = 11;
	              workInProgress = updateForwardRef(
	                null,
	                workInProgress,
	                current,
	                props,
	                renderLanes
	              );
	              break a;
	            } else if ($$typeof === REACT_MEMO_TYPE) {
	              workInProgress.tag = 14;
	              workInProgress = updateMemoComponent(
	                null,
	                workInProgress,
	                current,
	                props,
	                renderLanes
	              );
	              break a;
	            }
	          }
	          workInProgress = getComponentNameFromType(current) || current;
	          throw Error(formatProdErrorMessage(306, workInProgress, ""));
	        }
	      }
	      return workInProgress;
	    case 0:
	      return updateFunctionComponent(
	        current,
	        workInProgress,
	        workInProgress.type,
	        workInProgress.pendingProps,
	        renderLanes
	      );
	    case 1:
	      return (
	        (props = workInProgress.type),
	        ($$typeof = resolveClassComponentProps(
	          props,
	          workInProgress.pendingProps
	        )),
	        updateClassComponent(
	          current,
	          workInProgress,
	          props,
	          $$typeof,
	          renderLanes
	        )
	      );
	    case 3:
	      a: {
	        pushHostContainer(
	          workInProgress,
	          workInProgress.stateNode.containerInfo
	        );
	        if (null === current) throw Error(formatProdErrorMessage(387));
	        props = workInProgress.pendingProps;
	        var prevState = workInProgress.memoizedState;
	        $$typeof = prevState.element;
	        cloneUpdateQueue(current, workInProgress);
	        processUpdateQueue(workInProgress, props, null, renderLanes);
	        var nextState = workInProgress.memoizedState;
	        props = nextState.cache;
	        pushProvider(workInProgress, CacheContext, props);
	        props !== prevState.cache &&
	          propagateContextChanges(
	            workInProgress,
	            [CacheContext],
	            renderLanes,
	            !0
	          );
	        suspendIfUpdateReadFromEntangledAsyncAction();
	        props = nextState.element;
	        if (prevState.isDehydrated)
	          if (
	            ((prevState = {
	              element: props,
	              isDehydrated: !1,
	              cache: nextState.cache
	            }),
	            (workInProgress.updateQueue.baseState = prevState),
	            (workInProgress.memoizedState = prevState),
	            workInProgress.flags & 256)
	          ) {
	            workInProgress = mountHostRootWithoutHydrating(
	              current,
	              workInProgress,
	              props,
	              renderLanes
	            );
	            break a;
	          } else if (props !== $$typeof) {
	            $$typeof = createCapturedValueAtFiber(
	              Error(formatProdErrorMessage(424)),
	              workInProgress
	            );
	            queueHydrationError($$typeof);
	            workInProgress = mountHostRootWithoutHydrating(
	              current,
	              workInProgress,
	              props,
	              renderLanes
	            );
	            break a;
	          } else {
	            current = workInProgress.stateNode.containerInfo;
	            switch (current.nodeType) {
	              case 9:
	                current = current.body;
	                break;
	              default:
	                current =
	                  "HTML" === current.nodeName
	                    ? current.ownerDocument.body
	                    : current;
	            }
	            nextHydratableInstance = getNextHydratable(current.firstChild);
	            hydrationParentFiber = workInProgress;
	            isHydrating = !0;
	            hydrationErrors = null;
	            rootOrSingletonContext = !0;
	            renderLanes = mountChildFibers(
	              workInProgress,
	              null,
	              props,
	              renderLanes
	            );
	            for (workInProgress.child = renderLanes; renderLanes; )
	              (renderLanes.flags = (renderLanes.flags & -3) | 4096),
	                (renderLanes = renderLanes.sibling);
	          }
	        else {
	          resetHydrationState();
	          if (props === $$typeof) {
	            workInProgress = bailoutOnAlreadyFinishedWork(
	              current,
	              workInProgress,
	              renderLanes
	            );
	            break a;
	          }
	          reconcileChildren(current, workInProgress, props, renderLanes);
	        }
	        workInProgress = workInProgress.child;
	      }
	      return workInProgress;
	    case 26:
	      return (
	        markRef(current, workInProgress),
	        null === current
	          ? (renderLanes = getResource(
	              workInProgress.type,
	              null,
	              workInProgress.pendingProps,
	              null
	            ))
	            ? (workInProgress.memoizedState = renderLanes)
	            : isHydrating ||
	              ((renderLanes = workInProgress.type),
	              (current = workInProgress.pendingProps),
	              (props = getOwnerDocumentFromRootContainer(
	                rootInstanceStackCursor.current
	              ).createElement(renderLanes)),
	              (props[internalInstanceKey] = workInProgress),
	              (props[internalPropsKey] = current),
	              setInitialProperties(props, renderLanes, current),
	              markNodeAsHoistable(props),
	              (workInProgress.stateNode = props))
	          : (workInProgress.memoizedState = getResource(
	              workInProgress.type,
	              current.memoizedProps,
	              workInProgress.pendingProps,
	              current.memoizedState
	            )),
	        null
	      );
	    case 27:
	      return (
	        pushHostContext(workInProgress),
	        null === current &&
	          isHydrating &&
	          ((props = workInProgress.stateNode =
	            resolveSingletonInstance(
	              workInProgress.type,
	              workInProgress.pendingProps,
	              rootInstanceStackCursor.current
	            )),
	          (hydrationParentFiber = workInProgress),
	          (rootOrSingletonContext = !0),
	          ($$typeof = nextHydratableInstance),
	          isSingletonScope(workInProgress.type)
	            ? ((previousHydratableOnEnteringScopedSingleton = $$typeof),
	              (nextHydratableInstance = getNextHydratable(props.firstChild)))
	            : (nextHydratableInstance = $$typeof)),
	        reconcileChildren(
	          current,
	          workInProgress,
	          workInProgress.pendingProps.children,
	          renderLanes
	        ),
	        markRef(current, workInProgress),
	        null === current && (workInProgress.flags |= 4194304),
	        workInProgress.child
	      );
	    case 5:
	      if (null === current && isHydrating) {
	        if (($$typeof = props = nextHydratableInstance))
	          (props = canHydrateInstance(
	            props,
	            workInProgress.type,
	            workInProgress.pendingProps,
	            rootOrSingletonContext
	          )),
	            null !== props
	              ? ((workInProgress.stateNode = props),
	                (hydrationParentFiber = workInProgress),
	                (nextHydratableInstance = getNextHydratable(props.firstChild)),
	                (rootOrSingletonContext = !1),
	                ($$typeof = !0))
	              : ($$typeof = !1);
	        $$typeof || throwOnHydrationMismatch(workInProgress);
	      }
	      pushHostContext(workInProgress);
	      $$typeof = workInProgress.type;
	      prevState = workInProgress.pendingProps;
	      nextState = null !== current ? current.memoizedProps : null;
	      props = prevState.children;
	      shouldSetTextContent($$typeof, prevState)
	        ? (props = null)
	        : null !== nextState &&
	          shouldSetTextContent($$typeof, nextState) &&
	          (workInProgress.flags |= 32);
	      null !== workInProgress.memoizedState &&
	        (($$typeof = renderWithHooks(
	          current,
	          workInProgress,
	          TransitionAwareHostComponent,
	          null,
	          null,
	          renderLanes
	        )),
	        (HostTransitionContext._currentValue = $$typeof));
	      markRef(current, workInProgress);
	      reconcileChildren(current, workInProgress, props, renderLanes);
	      return workInProgress.child;
	    case 6:
	      if (null === current && isHydrating) {
	        if ((current = renderLanes = nextHydratableInstance))
	          (renderLanes = canHydrateTextInstance(
	            renderLanes,
	            workInProgress.pendingProps,
	            rootOrSingletonContext
	          )),
	            null !== renderLanes
	              ? ((workInProgress.stateNode = renderLanes),
	                (hydrationParentFiber = workInProgress),
	                (nextHydratableInstance = null),
	                (current = !0))
	              : (current = !1);
	        current || throwOnHydrationMismatch(workInProgress);
	      }
	      return null;
	    case 13:
	      return updateSuspenseComponent(current, workInProgress, renderLanes);
	    case 4:
	      return (
	        pushHostContainer(
	          workInProgress,
	          workInProgress.stateNode.containerInfo
	        ),
	        (props = workInProgress.pendingProps),
	        null === current
	          ? (workInProgress.child = reconcileChildFibers(
	              workInProgress,
	              null,
	              props,
	              renderLanes
	            ))
	          : reconcileChildren(current, workInProgress, props, renderLanes),
	        workInProgress.child
	      );
	    case 11:
	      return updateForwardRef(
	        current,
	        workInProgress,
	        workInProgress.type,
	        workInProgress.pendingProps,
	        renderLanes
	      );
	    case 7:
	      return (
	        reconcileChildren(
	          current,
	          workInProgress,
	          workInProgress.pendingProps,
	          renderLanes
	        ),
	        workInProgress.child
	      );
	    case 8:
	      return (
	        reconcileChildren(
	          current,
	          workInProgress,
	          workInProgress.pendingProps.children,
	          renderLanes
	        ),
	        workInProgress.child
	      );
	    case 12:
	      return (
	        reconcileChildren(
	          current,
	          workInProgress,
	          workInProgress.pendingProps.children,
	          renderLanes
	        ),
	        workInProgress.child
	      );
	    case 10:
	      return (
	        (props = workInProgress.pendingProps),
	        pushProvider(workInProgress, workInProgress.type, props.value),
	        reconcileChildren(current, workInProgress, props.children, renderLanes),
	        workInProgress.child
	      );
	    case 9:
	      return (
	        ($$typeof = workInProgress.type._context),
	        (props = workInProgress.pendingProps.children),
	        prepareToReadContext(workInProgress),
	        ($$typeof = readContext($$typeof)),
	        (props = props($$typeof)),
	        (workInProgress.flags |= 1),
	        reconcileChildren(current, workInProgress, props, renderLanes),
	        workInProgress.child
	      );
	    case 14:
	      return updateMemoComponent(
	        current,
	        workInProgress,
	        workInProgress.type,
	        workInProgress.pendingProps,
	        renderLanes
	      );
	    case 15:
	      return updateSimpleMemoComponent(
	        current,
	        workInProgress,
	        workInProgress.type,
	        workInProgress.pendingProps,
	        renderLanes
	      );
	    case 19:
	      return updateSuspenseListComponent(current, workInProgress, renderLanes);
	    case 31:
	      return updateActivityComponent(current, workInProgress, renderLanes);
	    case 22:
	      return updateOffscreenComponent(
	        current,
	        workInProgress,
	        renderLanes,
	        workInProgress.pendingProps
	      );
	    case 24:
	      return (
	        prepareToReadContext(workInProgress),
	        (props = readContext(CacheContext)),
	        null === current
	          ? (($$typeof = peekCacheFromPool()),
	            null === $$typeof &&
	              (($$typeof = workInProgressRoot),
	              (prevState = createCache()),
	              ($$typeof.pooledCache = prevState),
	              prevState.refCount++,
	              null !== prevState && ($$typeof.pooledCacheLanes |= renderLanes),
	              ($$typeof = prevState)),
	            (workInProgress.memoizedState = { parent: props, cache: $$typeof }),
	            initializeUpdateQueue(workInProgress),
	            pushProvider(workInProgress, CacheContext, $$typeof))
	          : (0 !== (current.lanes & renderLanes) &&
	              (cloneUpdateQueue(current, workInProgress),
	              processUpdateQueue(workInProgress, null, null, renderLanes),
	              suspendIfUpdateReadFromEntangledAsyncAction()),
	            ($$typeof = current.memoizedState),
	            (prevState = workInProgress.memoizedState),
	            $$typeof.parent !== props
	              ? (($$typeof = { parent: props, cache: props }),
	                (workInProgress.memoizedState = $$typeof),
	                0 === workInProgress.lanes &&
	                  (workInProgress.memoizedState =
	                    workInProgress.updateQueue.baseState =
	                      $$typeof),
	                pushProvider(workInProgress, CacheContext, props))
	              : ((props = prevState.cache),
	                pushProvider(workInProgress, CacheContext, props),
	                props !== $$typeof.cache &&
	                  propagateContextChanges(
	                    workInProgress,
	                    [CacheContext],
	                    renderLanes,
	                    !0
	                  ))),
	        reconcileChildren(
	          current,
	          workInProgress,
	          workInProgress.pendingProps.children,
	          renderLanes
	        ),
	        workInProgress.child
	      );
	    case 29:
	      throw workInProgress.pendingProps;
	  }
	  throw Error(formatProdErrorMessage(156, workInProgress.tag));
	}
	function markUpdate(workInProgress) {
	  workInProgress.flags |= 4;
	}
	function preloadInstanceAndSuspendIfNeeded(
	  workInProgress,
	  type,
	  oldProps,
	  newProps,
	  renderLanes
	) {
	  if ((type = 0 !== (workInProgress.mode & 32))) type = !1;
	  if (type) {
	    if (
	      ((workInProgress.flags |= 16777216),
	      (renderLanes & 335544128) === renderLanes)
	    )
	      if (workInProgress.stateNode.complete) workInProgress.flags |= 8192;
	      else if (shouldRemainOnPreviousScreen()) workInProgress.flags |= 8192;
	      else
	        throw (
	          ((suspendedThenable = noopSuspenseyCommitThenable),
	          SuspenseyCommitException)
	        );
	  } else workInProgress.flags &= -16777217;
	}
	function preloadResourceAndSuspendIfNeeded(workInProgress, resource) {
	  if ("stylesheet" !== resource.type || 0 !== (resource.state.loading & 4))
	    workInProgress.flags &= -16777217;
	  else if (((workInProgress.flags |= 16777216), !preloadResource(resource)))
	    if (shouldRemainOnPreviousScreen()) workInProgress.flags |= 8192;
	    else
	      throw (
	        ((suspendedThenable = noopSuspenseyCommitThenable),
	        SuspenseyCommitException)
	      );
	}
	function scheduleRetryEffect(workInProgress, retryQueue) {
	  null !== retryQueue && (workInProgress.flags |= 4);
	  workInProgress.flags & 16384 &&
	    ((retryQueue =
	      22 !== workInProgress.tag ? claimNextRetryLane() : 536870912),
	    (workInProgress.lanes |= retryQueue),
	    (workInProgressSuspendedRetryLanes |= retryQueue));
	}
	function cutOffTailIfNeeded(renderState, hasRenderedATailFallback) {
	  if (!isHydrating)
	    switch (renderState.tailMode) {
	      case "hidden":
	        hasRenderedATailFallback = renderState.tail;
	        for (var lastTailNode = null; null !== hasRenderedATailFallback; )
	          null !== hasRenderedATailFallback.alternate &&
	            (lastTailNode = hasRenderedATailFallback),
	            (hasRenderedATailFallback = hasRenderedATailFallback.sibling);
	        null === lastTailNode
	          ? (renderState.tail = null)
	          : (lastTailNode.sibling = null);
	        break;
	      case "collapsed":
	        lastTailNode = renderState.tail;
	        for (var lastTailNode$106 = null; null !== lastTailNode; )
	          null !== lastTailNode.alternate && (lastTailNode$106 = lastTailNode),
	            (lastTailNode = lastTailNode.sibling);
	        null === lastTailNode$106
	          ? hasRenderedATailFallback || null === renderState.tail
	            ? (renderState.tail = null)
	            : (renderState.tail.sibling = null)
	          : (lastTailNode$106.sibling = null);
	    }
	}
	function bubbleProperties(completedWork) {
	  var didBailout =
	      null !== completedWork.alternate &&
	      completedWork.alternate.child === completedWork.child,
	    newChildLanes = 0,
	    subtreeFlags = 0;
	  if (didBailout)
	    for (var child$107 = completedWork.child; null !== child$107; )
	      (newChildLanes |= child$107.lanes | child$107.childLanes),
	        (subtreeFlags |= child$107.subtreeFlags & 65011712),
	        (subtreeFlags |= child$107.flags & 65011712),
	        (child$107.return = completedWork),
	        (child$107 = child$107.sibling);
	  else
	    for (child$107 = completedWork.child; null !== child$107; )
	      (newChildLanes |= child$107.lanes | child$107.childLanes),
	        (subtreeFlags |= child$107.subtreeFlags),
	        (subtreeFlags |= child$107.flags),
	        (child$107.return = completedWork),
	        (child$107 = child$107.sibling);
	  completedWork.subtreeFlags |= subtreeFlags;
	  completedWork.childLanes = newChildLanes;
	  return didBailout;
	}
	function completeWork(current, workInProgress, renderLanes) {
	  var newProps = workInProgress.pendingProps;
	  popTreeContext(workInProgress);
	  switch (workInProgress.tag) {
	    case 16:
	    case 15:
	    case 0:
	    case 11:
	    case 7:
	    case 8:
	    case 12:
	    case 9:
	    case 14:
	      return bubbleProperties(workInProgress), null;
	    case 1:
	      return bubbleProperties(workInProgress), null;
	    case 3:
	      renderLanes = workInProgress.stateNode;
	      newProps = null;
	      null !== current && (newProps = current.memoizedState.cache);
	      workInProgress.memoizedState.cache !== newProps &&
	        (workInProgress.flags |= 2048);
	      popProvider(CacheContext);
	      popHostContainer();
	      renderLanes.pendingContext &&
	        ((renderLanes.context = renderLanes.pendingContext),
	        (renderLanes.pendingContext = null));
	      if (null === current || null === current.child)
	        popHydrationState(workInProgress)
	          ? markUpdate(workInProgress)
	          : null === current ||
	            (current.memoizedState.isDehydrated &&
	              0 === (workInProgress.flags & 256)) ||
	            ((workInProgress.flags |= 1024),
	            upgradeHydrationErrorsToRecoverable());
	      bubbleProperties(workInProgress);
	      return null;
	    case 26:
	      var type = workInProgress.type,
	        nextResource = workInProgress.memoizedState;
	      null === current
	        ? (markUpdate(workInProgress),
	          null !== nextResource
	            ? (bubbleProperties(workInProgress),
	              preloadResourceAndSuspendIfNeeded(workInProgress, nextResource))
	            : (bubbleProperties(workInProgress),
	              preloadInstanceAndSuspendIfNeeded(
	                workInProgress,
	                type,
	                null,
	                newProps,
	                renderLanes
	              )))
	        : nextResource
	          ? nextResource !== current.memoizedState
	            ? (markUpdate(workInProgress),
	              bubbleProperties(workInProgress),
	              preloadResourceAndSuspendIfNeeded(workInProgress, nextResource))
	            : (bubbleProperties(workInProgress),
	              (workInProgress.flags &= -16777217))
	          : ((current = current.memoizedProps),
	            current !== newProps && markUpdate(workInProgress),
	            bubbleProperties(workInProgress),
	            preloadInstanceAndSuspendIfNeeded(
	              workInProgress,
	              type,
	              current,
	              newProps,
	              renderLanes
	            ));
	      return null;
	    case 27:
	      popHostContext(workInProgress);
	      renderLanes = rootInstanceStackCursor.current;
	      type = workInProgress.type;
	      if (null !== current && null != workInProgress.stateNode)
	        current.memoizedProps !== newProps && markUpdate(workInProgress);
	      else {
	        if (!newProps) {
	          if (null === workInProgress.stateNode)
	            throw Error(formatProdErrorMessage(166));
	          bubbleProperties(workInProgress);
	          return null;
	        }
	        current = contextStackCursor.current;
	        popHydrationState(workInProgress)
	          ? prepareToHydrateHostInstance(workInProgress, current)
	          : ((current = resolveSingletonInstance(type, newProps, renderLanes)),
	            (workInProgress.stateNode = current),
	            markUpdate(workInProgress));
	      }
	      bubbleProperties(workInProgress);
	      return null;
	    case 5:
	      popHostContext(workInProgress);
	      type = workInProgress.type;
	      if (null !== current && null != workInProgress.stateNode)
	        current.memoizedProps !== newProps && markUpdate(workInProgress);
	      else {
	        if (!newProps) {
	          if (null === workInProgress.stateNode)
	            throw Error(formatProdErrorMessage(166));
	          bubbleProperties(workInProgress);
	          return null;
	        }
	        nextResource = contextStackCursor.current;
	        if (popHydrationState(workInProgress))
	          prepareToHydrateHostInstance(workInProgress, nextResource);
	        else {
	          var ownerDocument = getOwnerDocumentFromRootContainer(
	            rootInstanceStackCursor.current
	          );
	          switch (nextResource) {
	            case 1:
	              nextResource = ownerDocument.createElementNS(
	                "http://www.w3.org/2000/svg",
	                type
	              );
	              break;
	            case 2:
	              nextResource = ownerDocument.createElementNS(
	                "http://www.w3.org/1998/Math/MathML",
	                type
	              );
	              break;
	            default:
	              switch (type) {
	                case "svg":
	                  nextResource = ownerDocument.createElementNS(
	                    "http://www.w3.org/2000/svg",
	                    type
	                  );
	                  break;
	                case "math":
	                  nextResource = ownerDocument.createElementNS(
	                    "http://www.w3.org/1998/Math/MathML",
	                    type
	                  );
	                  break;
	                case "script":
	                  nextResource = ownerDocument.createElement("div");
	                  nextResource.innerHTML = "<script>\x3c/script>";
	                  nextResource = nextResource.removeChild(
	                    nextResource.firstChild
	                  );
	                  break;
	                case "select":
	                  nextResource =
	                    "string" === typeof newProps.is
	                      ? ownerDocument.createElement("select", {
	                          is: newProps.is
	                        })
	                      : ownerDocument.createElement("select");
	                  newProps.multiple
	                    ? (nextResource.multiple = !0)
	                    : newProps.size && (nextResource.size = newProps.size);
	                  break;
	                default:
	                  nextResource =
	                    "string" === typeof newProps.is
	                      ? ownerDocument.createElement(type, { is: newProps.is })
	                      : ownerDocument.createElement(type);
	              }
	          }
	          nextResource[internalInstanceKey] = workInProgress;
	          nextResource[internalPropsKey] = newProps;
	          a: for (
	            ownerDocument = workInProgress.child;
	            null !== ownerDocument;

	          ) {
	            if (5 === ownerDocument.tag || 6 === ownerDocument.tag)
	              nextResource.appendChild(ownerDocument.stateNode);
	            else if (
	              4 !== ownerDocument.tag &&
	              27 !== ownerDocument.tag &&
	              null !== ownerDocument.child
	            ) {
	              ownerDocument.child.return = ownerDocument;
	              ownerDocument = ownerDocument.child;
	              continue;
	            }
	            if (ownerDocument === workInProgress) break a;
	            for (; null === ownerDocument.sibling; ) {
	              if (
	                null === ownerDocument.return ||
	                ownerDocument.return === workInProgress
	              )
	                break a;
	              ownerDocument = ownerDocument.return;
	            }
	            ownerDocument.sibling.return = ownerDocument.return;
	            ownerDocument = ownerDocument.sibling;
	          }
	          workInProgress.stateNode = nextResource;
	          a: switch (
	            (setInitialProperties(nextResource, type, newProps), type)
	          ) {
	            case "button":
	            case "input":
	            case "select":
	            case "textarea":
	              newProps = !!newProps.autoFocus;
	              break a;
	            case "img":
	              newProps = !0;
	              break a;
	            default:
	              newProps = !1;
	          }
	          newProps && markUpdate(workInProgress);
	        }
	      }
	      bubbleProperties(workInProgress);
	      preloadInstanceAndSuspendIfNeeded(
	        workInProgress,
	        workInProgress.type,
	        null === current ? null : current.memoizedProps,
	        workInProgress.pendingProps,
	        renderLanes
	      );
	      return null;
	    case 6:
	      if (current && null != workInProgress.stateNode)
	        current.memoizedProps !== newProps && markUpdate(workInProgress);
	      else {
	        if ("string" !== typeof newProps && null === workInProgress.stateNode)
	          throw Error(formatProdErrorMessage(166));
	        current = rootInstanceStackCursor.current;
	        if (popHydrationState(workInProgress)) {
	          current = workInProgress.stateNode;
	          renderLanes = workInProgress.memoizedProps;
	          newProps = null;
	          type = hydrationParentFiber;
	          if (null !== type)
	            switch (type.tag) {
	              case 27:
	              case 5:
	                newProps = type.memoizedProps;
	            }
	          current[internalInstanceKey] = workInProgress;
	          current =
	            current.nodeValue === renderLanes ||
	            (null !== newProps && !0 === newProps.suppressHydrationWarning) ||
	            checkForUnmatchedText(current.nodeValue, renderLanes)
	              ? !0
	              : !1;
	          current || throwOnHydrationMismatch(workInProgress, !0);
	        } else
	          (current =
	            getOwnerDocumentFromRootContainer(current).createTextNode(
	              newProps
	            )),
	            (current[internalInstanceKey] = workInProgress),
	            (workInProgress.stateNode = current);
	      }
	      bubbleProperties(workInProgress);
	      return null;
	    case 31:
	      renderLanes = workInProgress.memoizedState;
	      if (null === current || null !== current.memoizedState) {
	        newProps = popHydrationState(workInProgress);
	        if (null !== renderLanes) {
	          if (null === current) {
	            if (!newProps) throw Error(formatProdErrorMessage(318));
	            current = workInProgress.memoizedState;
	            current = null !== current ? current.dehydrated : null;
	            if (!current) throw Error(formatProdErrorMessage(557));
	            current[internalInstanceKey] = workInProgress;
	          } else
	            resetHydrationState(),
	              0 === (workInProgress.flags & 128) &&
	                (workInProgress.memoizedState = null),
	              (workInProgress.flags |= 4);
	          bubbleProperties(workInProgress);
	          current = !1;
	        } else
	          (renderLanes = upgradeHydrationErrorsToRecoverable()),
	            null !== current &&
	              null !== current.memoizedState &&
	              (current.memoizedState.hydrationErrors = renderLanes),
	            (current = !0);
	        if (!current) {
	          if (workInProgress.flags & 256)
	            return popSuspenseHandler(workInProgress), workInProgress;
	          popSuspenseHandler(workInProgress);
	          return null;
	        }
	        if (0 !== (workInProgress.flags & 128))
	          throw Error(formatProdErrorMessage(558));
	      }
	      bubbleProperties(workInProgress);
	      return null;
	    case 13:
	      newProps = workInProgress.memoizedState;
	      if (
	        null === current ||
	        (null !== current.memoizedState &&
	          null !== current.memoizedState.dehydrated)
	      ) {
	        type = popHydrationState(workInProgress);
	        if (null !== newProps && null !== newProps.dehydrated) {
	          if (null === current) {
	            if (!type) throw Error(formatProdErrorMessage(318));
	            type = workInProgress.memoizedState;
	            type = null !== type ? type.dehydrated : null;
	            if (!type) throw Error(formatProdErrorMessage(317));
	            type[internalInstanceKey] = workInProgress;
	          } else
	            resetHydrationState(),
	              0 === (workInProgress.flags & 128) &&
	                (workInProgress.memoizedState = null),
	              (workInProgress.flags |= 4);
	          bubbleProperties(workInProgress);
	          type = !1;
	        } else
	          (type = upgradeHydrationErrorsToRecoverable()),
	            null !== current &&
	              null !== current.memoizedState &&
	              (current.memoizedState.hydrationErrors = type),
	            (type = !0);
	        if (!type) {
	          if (workInProgress.flags & 256)
	            return popSuspenseHandler(workInProgress), workInProgress;
	          popSuspenseHandler(workInProgress);
	          return null;
	        }
	      }
	      popSuspenseHandler(workInProgress);
	      if (0 !== (workInProgress.flags & 128))
	        return (workInProgress.lanes = renderLanes), workInProgress;
	      renderLanes = null !== newProps;
	      current = null !== current && null !== current.memoizedState;
	      renderLanes &&
	        ((newProps = workInProgress.child),
	        (type = null),
	        null !== newProps.alternate &&
	          null !== newProps.alternate.memoizedState &&
	          null !== newProps.alternate.memoizedState.cachePool &&
	          (type = newProps.alternate.memoizedState.cachePool.pool),
	        (nextResource = null),
	        null !== newProps.memoizedState &&
	          null !== newProps.memoizedState.cachePool &&
	          (nextResource = newProps.memoizedState.cachePool.pool),
	        nextResource !== type && (newProps.flags |= 2048));
	      renderLanes !== current &&
	        renderLanes &&
	        (workInProgress.child.flags |= 8192);
	      scheduleRetryEffect(workInProgress, workInProgress.updateQueue);
	      bubbleProperties(workInProgress);
	      return null;
	    case 4:
	      return (
	        popHostContainer(),
	        null === current &&
	          listenToAllSupportedEvents(workInProgress.stateNode.containerInfo),
	        bubbleProperties(workInProgress),
	        null
	      );
	    case 10:
	      return (
	        popProvider(workInProgress.type), bubbleProperties(workInProgress), null
	      );
	    case 19:
	      pop(suspenseStackCursor);
	      newProps = workInProgress.memoizedState;
	      if (null === newProps) return bubbleProperties(workInProgress), null;
	      type = 0 !== (workInProgress.flags & 128);
	      nextResource = newProps.rendering;
	      if (null === nextResource)
	        if (type) cutOffTailIfNeeded(newProps, !1);
	        else {
	          if (
	            0 !== workInProgressRootExitStatus ||
	            (null !== current && 0 !== (current.flags & 128))
	          )
	            for (current = workInProgress.child; null !== current; ) {
	              nextResource = findFirstSuspended(current);
	              if (null !== nextResource) {
	                workInProgress.flags |= 128;
	                cutOffTailIfNeeded(newProps, !1);
	                current = nextResource.updateQueue;
	                workInProgress.updateQueue = current;
	                scheduleRetryEffect(workInProgress, current);
	                workInProgress.subtreeFlags = 0;
	                current = renderLanes;
	                for (renderLanes = workInProgress.child; null !== renderLanes; )
	                  resetWorkInProgress(renderLanes, current),
	                    (renderLanes = renderLanes.sibling);
	                push(
	                  suspenseStackCursor,
	                  (suspenseStackCursor.current & 1) | 2
	                );
	                isHydrating &&
	                  pushTreeFork(workInProgress, newProps.treeForkCount);
	                return workInProgress.child;
	              }
	              current = current.sibling;
	            }
	          null !== newProps.tail &&
	            now() > workInProgressRootRenderTargetTime &&
	            ((workInProgress.flags |= 128),
	            (type = !0),
	            cutOffTailIfNeeded(newProps, !1),
	            (workInProgress.lanes = 4194304));
	        }
	      else {
	        if (!type)
	          if (
	            ((current = findFirstSuspended(nextResource)), null !== current)
	          ) {
	            if (
	              ((workInProgress.flags |= 128),
	              (type = !0),
	              (current = current.updateQueue),
	              (workInProgress.updateQueue = current),
	              scheduleRetryEffect(workInProgress, current),
	              cutOffTailIfNeeded(newProps, !0),
	              null === newProps.tail &&
	                "hidden" === newProps.tailMode &&
	                !nextResource.alternate &&
	                !isHydrating)
	            )
	              return bubbleProperties(workInProgress), null;
	          } else
	            2 * now() - newProps.renderingStartTime >
	              workInProgressRootRenderTargetTime &&
	              536870912 !== renderLanes &&
	              ((workInProgress.flags |= 128),
	              (type = !0),
	              cutOffTailIfNeeded(newProps, !1),
	              (workInProgress.lanes = 4194304));
	        newProps.isBackwards
	          ? ((nextResource.sibling = workInProgress.child),
	            (workInProgress.child = nextResource))
	          : ((current = newProps.last),
	            null !== current
	              ? (current.sibling = nextResource)
	              : (workInProgress.child = nextResource),
	            (newProps.last = nextResource));
	      }
	      if (null !== newProps.tail)
	        return (
	          (current = newProps.tail),
	          (newProps.rendering = current),
	          (newProps.tail = current.sibling),
	          (newProps.renderingStartTime = now()),
	          (current.sibling = null),
	          (renderLanes = suspenseStackCursor.current),
	          push(
	            suspenseStackCursor,
	            type ? (renderLanes & 1) | 2 : renderLanes & 1
	          ),
	          isHydrating && pushTreeFork(workInProgress, newProps.treeForkCount),
	          current
	        );
	      bubbleProperties(workInProgress);
	      return null;
	    case 22:
	    case 23:
	      return (
	        popSuspenseHandler(workInProgress),
	        popHiddenContext(),
	        (newProps = null !== workInProgress.memoizedState),
	        null !== current
	          ? (null !== current.memoizedState) !== newProps &&
	            (workInProgress.flags |= 8192)
	          : newProps && (workInProgress.flags |= 8192),
	        newProps
	          ? 0 !== (renderLanes & 536870912) &&
	            0 === (workInProgress.flags & 128) &&
	            (bubbleProperties(workInProgress),
	            workInProgress.subtreeFlags & 6 && (workInProgress.flags |= 8192))
	          : bubbleProperties(workInProgress),
	        (renderLanes = workInProgress.updateQueue),
	        null !== renderLanes &&
	          scheduleRetryEffect(workInProgress, renderLanes.retryQueue),
	        (renderLanes = null),
	        null !== current &&
	          null !== current.memoizedState &&
	          null !== current.memoizedState.cachePool &&
	          (renderLanes = current.memoizedState.cachePool.pool),
	        (newProps = null),
	        null !== workInProgress.memoizedState &&
	          null !== workInProgress.memoizedState.cachePool &&
	          (newProps = workInProgress.memoizedState.cachePool.pool),
	        newProps !== renderLanes && (workInProgress.flags |= 2048),
	        null !== current && pop(resumedCache),
	        null
	      );
	    case 24:
	      return (
	        (renderLanes = null),
	        null !== current && (renderLanes = current.memoizedState.cache),
	        workInProgress.memoizedState.cache !== renderLanes &&
	          (workInProgress.flags |= 2048),
	        popProvider(CacheContext),
	        bubbleProperties(workInProgress),
	        null
	      );
	    case 25:
	      return null;
	    case 30:
	      return null;
	  }
	  throw Error(formatProdErrorMessage(156, workInProgress.tag));
	}
	function unwindWork(current, workInProgress) {
	  popTreeContext(workInProgress);
	  switch (workInProgress.tag) {
	    case 1:
	      return (
	        (current = workInProgress.flags),
	        current & 65536
	          ? ((workInProgress.flags = (current & -65537) | 128), workInProgress)
	          : null
	      );
	    case 3:
	      return (
	        popProvider(CacheContext),
	        popHostContainer(),
	        (current = workInProgress.flags),
	        0 !== (current & 65536) && 0 === (current & 128)
	          ? ((workInProgress.flags = (current & -65537) | 128), workInProgress)
	          : null
	      );
	    case 26:
	    case 27:
	    case 5:
	      return popHostContext(workInProgress), null;
	    case 31:
	      if (null !== workInProgress.memoizedState) {
	        popSuspenseHandler(workInProgress);
	        if (null === workInProgress.alternate)
	          throw Error(formatProdErrorMessage(340));
	        resetHydrationState();
	      }
	      current = workInProgress.flags;
	      return current & 65536
	        ? ((workInProgress.flags = (current & -65537) | 128), workInProgress)
	        : null;
	    case 13:
	      popSuspenseHandler(workInProgress);
	      current = workInProgress.memoizedState;
	      if (null !== current && null !== current.dehydrated) {
	        if (null === workInProgress.alternate)
	          throw Error(formatProdErrorMessage(340));
	        resetHydrationState();
	      }
	      current = workInProgress.flags;
	      return current & 65536
	        ? ((workInProgress.flags = (current & -65537) | 128), workInProgress)
	        : null;
	    case 19:
	      return pop(suspenseStackCursor), null;
	    case 4:
	      return popHostContainer(), null;
	    case 10:
	      return popProvider(workInProgress.type), null;
	    case 22:
	    case 23:
	      return (
	        popSuspenseHandler(workInProgress),
	        popHiddenContext(),
	        null !== current && pop(resumedCache),
	        (current = workInProgress.flags),
	        current & 65536
	          ? ((workInProgress.flags = (current & -65537) | 128), workInProgress)
	          : null
	      );
	    case 24:
	      return popProvider(CacheContext), null;
	    case 25:
	      return null;
	    default:
	      return null;
	  }
	}
	function unwindInterruptedWork(current, interruptedWork) {
	  popTreeContext(interruptedWork);
	  switch (interruptedWork.tag) {
	    case 3:
	      popProvider(CacheContext);
	      popHostContainer();
	      break;
	    case 26:
	    case 27:
	    case 5:
	      popHostContext(interruptedWork);
	      break;
	    case 4:
	      popHostContainer();
	      break;
	    case 31:
	      null !== interruptedWork.memoizedState &&
	        popSuspenseHandler(interruptedWork);
	      break;
	    case 13:
	      popSuspenseHandler(interruptedWork);
	      break;
	    case 19:
	      pop(suspenseStackCursor);
	      break;
	    case 10:
	      popProvider(interruptedWork.type);
	      break;
	    case 22:
	    case 23:
	      popSuspenseHandler(interruptedWork);
	      popHiddenContext();
	      null !== current && pop(resumedCache);
	      break;
	    case 24:
	      popProvider(CacheContext);
	  }
	}
	function commitHookEffectListMount(flags, finishedWork) {
	  try {
	    var updateQueue = finishedWork.updateQueue,
	      lastEffect = null !== updateQueue ? updateQueue.lastEffect : null;
	    if (null !== lastEffect) {
	      var firstEffect = lastEffect.next;
	      updateQueue = firstEffect;
	      do {
	        if ((updateQueue.tag & flags) === flags) {
	          lastEffect = void 0;
	          var create = updateQueue.create,
	            inst = updateQueue.inst;
	          lastEffect = create();
	          inst.destroy = lastEffect;
	        }
	        updateQueue = updateQueue.next;
	      } while (updateQueue !== firstEffect);
	    }
	  } catch (error) {
	    captureCommitPhaseError(finishedWork, finishedWork.return, error);
	  }
	}
	function commitHookEffectListUnmount(
	  flags,
	  finishedWork,
	  nearestMountedAncestor$jscomp$0
	) {
	  try {
	    var updateQueue = finishedWork.updateQueue,
	      lastEffect = null !== updateQueue ? updateQueue.lastEffect : null;
	    if (null !== lastEffect) {
	      var firstEffect = lastEffect.next;
	      updateQueue = firstEffect;
	      do {
	        if ((updateQueue.tag & flags) === flags) {
	          var inst = updateQueue.inst,
	            destroy = inst.destroy;
	          if (void 0 !== destroy) {
	            inst.destroy = void 0;
	            lastEffect = finishedWork;
	            var nearestMountedAncestor = nearestMountedAncestor$jscomp$0,
	              destroy_ = destroy;
	            try {
	              destroy_();
	            } catch (error) {
	              captureCommitPhaseError(
	                lastEffect,
	                nearestMountedAncestor,
	                error
	              );
	            }
	          }
	        }
	        updateQueue = updateQueue.next;
	      } while (updateQueue !== firstEffect);
	    }
	  } catch (error) {
	    captureCommitPhaseError(finishedWork, finishedWork.return, error);
	  }
	}
	function commitClassCallbacks(finishedWork) {
	  var updateQueue = finishedWork.updateQueue;
	  if (null !== updateQueue) {
	    var instance = finishedWork.stateNode;
	    try {
	      commitCallbacks(updateQueue, instance);
	    } catch (error) {
	      captureCommitPhaseError(finishedWork, finishedWork.return, error);
	    }
	  }
	}
	function safelyCallComponentWillUnmount(
	  current,
	  nearestMountedAncestor,
	  instance
	) {
	  instance.props = resolveClassComponentProps(
	    current.type,
	    current.memoizedProps
	  );
	  instance.state = current.memoizedState;
	  try {
	    instance.componentWillUnmount();
	  } catch (error) {
	    captureCommitPhaseError(current, nearestMountedAncestor, error);
	  }
	}
	function safelyAttachRef(current, nearestMountedAncestor) {
	  try {
	    var ref = current.ref;
	    if (null !== ref) {
	      switch (current.tag) {
	        case 26:
	        case 27:
	        case 5:
	          var instanceToUse = current.stateNode;
	          break;
	        case 30:
	          instanceToUse = current.stateNode;
	          break;
	        default:
	          instanceToUse = current.stateNode;
	      }
	      "function" === typeof ref
	        ? (current.refCleanup = ref(instanceToUse))
	        : (ref.current = instanceToUse);
	    }
	  } catch (error) {
	    captureCommitPhaseError(current, nearestMountedAncestor, error);
	  }
	}
	function safelyDetachRef(current, nearestMountedAncestor) {
	  var ref = current.ref,
	    refCleanup = current.refCleanup;
	  if (null !== ref)
	    if ("function" === typeof refCleanup)
	      try {
	        refCleanup();
	      } catch (error) {
	        captureCommitPhaseError(current, nearestMountedAncestor, error);
	      } finally {
	        (current.refCleanup = null),
	          (current = current.alternate),
	          null != current && (current.refCleanup = null);
	      }
	    else if ("function" === typeof ref)
	      try {
	        ref(null);
	      } catch (error$140) {
	        captureCommitPhaseError(current, nearestMountedAncestor, error$140);
	      }
	    else ref.current = null;
	}
	function commitHostMount(finishedWork) {
	  var type = finishedWork.type,
	    props = finishedWork.memoizedProps,
	    instance = finishedWork.stateNode;
	  try {
	    a: switch (type) {
	      case "button":
	      case "input":
	      case "select":
	      case "textarea":
	        props.autoFocus && instance.focus();
	        break a;
	      case "img":
	        props.src
	          ? (instance.src = props.src)
	          : props.srcSet && (instance.srcset = props.srcSet);
	    }
	  } catch (error) {
	    captureCommitPhaseError(finishedWork, finishedWork.return, error);
	  }
	}
	function commitHostUpdate(finishedWork, newProps, oldProps) {
	  try {
	    var domElement = finishedWork.stateNode;
	    updateProperties(domElement, finishedWork.type, oldProps, newProps);
	    domElement[internalPropsKey] = newProps;
	  } catch (error) {
	    captureCommitPhaseError(finishedWork, finishedWork.return, error);
	  }
	}
	function isHostParent(fiber) {
	  return (
	    5 === fiber.tag ||
	    3 === fiber.tag ||
	    26 === fiber.tag ||
	    (27 === fiber.tag && isSingletonScope(fiber.type)) ||
	    4 === fiber.tag
	  );
	}
	function getHostSibling(fiber) {
	  a: for (;;) {
	    for (; null === fiber.sibling; ) {
	      if (null === fiber.return || isHostParent(fiber.return)) return null;
	      fiber = fiber.return;
	    }
	    fiber.sibling.return = fiber.return;
	    for (
	      fiber = fiber.sibling;
	      5 !== fiber.tag && 6 !== fiber.tag && 18 !== fiber.tag;

	    ) {
	      if (27 === fiber.tag && isSingletonScope(fiber.type)) continue a;
	      if (fiber.flags & 2) continue a;
	      if (null === fiber.child || 4 === fiber.tag) continue a;
	      else (fiber.child.return = fiber), (fiber = fiber.child);
	    }
	    if (!(fiber.flags & 2)) return fiber.stateNode;
	  }
	}
	function insertOrAppendPlacementNodeIntoContainer(node, before, parent) {
	  var tag = node.tag;
	  if (5 === tag || 6 === tag)
	    (node = node.stateNode),
	      before
	        ? (9 === parent.nodeType
	            ? parent.body
	            : "HTML" === parent.nodeName
	              ? parent.ownerDocument.body
	              : parent
	          ).insertBefore(node, before)
	        : ((before =
	            9 === parent.nodeType
	              ? parent.body
	              : "HTML" === parent.nodeName
	                ? parent.ownerDocument.body
	                : parent),
	          before.appendChild(node),
	          (parent = parent._reactRootContainer),
	          (null !== parent && void 0 !== parent) ||
	            null !== before.onclick ||
	            (before.onclick = noop$1));
	  else if (
	    4 !== tag &&
	    (27 === tag &&
	      isSingletonScope(node.type) &&
	      ((parent = node.stateNode), (before = null)),
	    (node = node.child),
	    null !== node)
	  )
	    for (
	      insertOrAppendPlacementNodeIntoContainer(node, before, parent),
	        node = node.sibling;
	      null !== node;

	    )
	      insertOrAppendPlacementNodeIntoContainer(node, before, parent),
	        (node = node.sibling);
	}
	function insertOrAppendPlacementNode(node, before, parent) {
	  var tag = node.tag;
	  if (5 === tag || 6 === tag)
	    (node = node.stateNode),
	      before ? parent.insertBefore(node, before) : parent.appendChild(node);
	  else if (
	    4 !== tag &&
	    (27 === tag && isSingletonScope(node.type) && (parent = node.stateNode),
	    (node = node.child),
	    null !== node)
	  )
	    for (
	      insertOrAppendPlacementNode(node, before, parent), node = node.sibling;
	      null !== node;

	    )
	      insertOrAppendPlacementNode(node, before, parent), (node = node.sibling);
	}
	function commitHostSingletonAcquisition(finishedWork) {
	  var singleton = finishedWork.stateNode,
	    props = finishedWork.memoizedProps;
	  try {
	    for (
	      var type = finishedWork.type, attributes = singleton.attributes;
	      attributes.length;

	    )
	      singleton.removeAttributeNode(attributes[0]);
	    setInitialProperties(singleton, type, props);
	    singleton[internalInstanceKey] = finishedWork;
	    singleton[internalPropsKey] = props;
	  } catch (error) {
	    captureCommitPhaseError(finishedWork, finishedWork.return, error);
	  }
	}
	var offscreenSubtreeIsHidden = !1,
	  offscreenSubtreeWasHidden = !1,
	  needsFormReset = !1,
	  PossiblyWeakSet = "function" === typeof WeakSet ? WeakSet : Set,
	  nextEffect = null;
	function commitBeforeMutationEffects(root, firstChild) {
	  root = root.containerInfo;
	  eventsEnabled = _enabled;
	  root = getActiveElementDeep(root);
	  if (hasSelectionCapabilities(root)) {
	    if ("selectionStart" in root)
	      var JSCompiler_temp = {
	        start: root.selectionStart,
	        end: root.selectionEnd
	      };
	    else
	      a: {
	        JSCompiler_temp =
	          ((JSCompiler_temp = root.ownerDocument) &&
	            JSCompiler_temp.defaultView) ||
	          window;
	        var selection =
	          JSCompiler_temp.getSelection && JSCompiler_temp.getSelection();
	        if (selection && 0 !== selection.rangeCount) {
	          JSCompiler_temp = selection.anchorNode;
	          var anchorOffset = selection.anchorOffset,
	            focusNode = selection.focusNode;
	          selection = selection.focusOffset;
	          try {
	            JSCompiler_temp.nodeType, focusNode.nodeType;
	          } catch (e$20) {
	            JSCompiler_temp = null;
	            break a;
	          }
	          var length = 0,
	            start = -1,
	            end = -1,
	            indexWithinAnchor = 0,
	            indexWithinFocus = 0,
	            node = root,
	            parentNode = null;
	          b: for (;;) {
	            for (var next; ; ) {
	              node !== JSCompiler_temp ||
	                (0 !== anchorOffset && 3 !== node.nodeType) ||
	                (start = length + anchorOffset);
	              node !== focusNode ||
	                (0 !== selection && 3 !== node.nodeType) ||
	                (end = length + selection);
	              3 === node.nodeType && (length += node.nodeValue.length);
	              if (null === (next = node.firstChild)) break;
	              parentNode = node;
	              node = next;
	            }
	            for (;;) {
	              if (node === root) break b;
	              parentNode === JSCompiler_temp &&
	                ++indexWithinAnchor === anchorOffset &&
	                (start = length);
	              parentNode === focusNode &&
	                ++indexWithinFocus === selection &&
	                (end = length);
	              if (null !== (next = node.nextSibling)) break;
	              node = parentNode;
	              parentNode = node.parentNode;
	            }
	            node = next;
	          }
	          JSCompiler_temp =
	            -1 === start || -1 === end ? null : { start: start, end: end };
	        } else JSCompiler_temp = null;
	      }
	    JSCompiler_temp = JSCompiler_temp || { start: 0, end: 0 };
	  } else JSCompiler_temp = null;
	  selectionInformation = { focusedElem: root, selectionRange: JSCompiler_temp };
	  _enabled = !1;
	  for (nextEffect = firstChild; null !== nextEffect; )
	    if (
	      ((firstChild = nextEffect),
	      (root = firstChild.child),
	      0 !== (firstChild.subtreeFlags & 1028) && null !== root)
	    )
	      (root.return = firstChild), (nextEffect = root);
	    else
	      for (; null !== nextEffect; ) {
	        firstChild = nextEffect;
	        focusNode = firstChild.alternate;
	        root = firstChild.flags;
	        switch (firstChild.tag) {
	          case 0:
	            if (
	              0 !== (root & 4) &&
	              ((root = firstChild.updateQueue),
	              (root = null !== root ? root.events : null),
	              null !== root)
	            )
	              for (
	                JSCompiler_temp = 0;
	                JSCompiler_temp < root.length;
	                JSCompiler_temp++
	              )
	                (anchorOffset = root[JSCompiler_temp]),
	                  (anchorOffset.ref.impl = anchorOffset.nextImpl);
	            break;
	          case 11:
	          case 15:
	            break;
	          case 1:
	            if (0 !== (root & 1024) && null !== focusNode) {
	              root = void 0;
	              JSCompiler_temp = firstChild;
	              anchorOffset = focusNode.memoizedProps;
	              focusNode = focusNode.memoizedState;
	              selection = JSCompiler_temp.stateNode;
	              try {
	                var resolvedPrevProps = resolveClassComponentProps(
	                  JSCompiler_temp.type,
	                  anchorOffset
	                );
	                root = selection.getSnapshotBeforeUpdate(
	                  resolvedPrevProps,
	                  focusNode
	                );
	                selection.__reactInternalSnapshotBeforeUpdate = root;
	              } catch (error) {
	                captureCommitPhaseError(
	                  JSCompiler_temp,
	                  JSCompiler_temp.return,
	                  error
	                );
	              }
	            }
	            break;
	          case 3:
	            if (0 !== (root & 1024))
	              if (
	                ((root = firstChild.stateNode.containerInfo),
	                (JSCompiler_temp = root.nodeType),
	                9 === JSCompiler_temp)
	              )
	                clearContainerSparingly(root);
	              else if (1 === JSCompiler_temp)
	                switch (root.nodeName) {
	                  case "HEAD":
	                  case "HTML":
	                  case "BODY":
	                    clearContainerSparingly(root);
	                    break;
	                  default:
	                    root.textContent = "";
	                }
	            break;
	          case 5:
	          case 26:
	          case 27:
	          case 6:
	          case 4:
	          case 17:
	            break;
	          default:
	            if (0 !== (root & 1024)) throw Error(formatProdErrorMessage(163));
	        }
	        root = firstChild.sibling;
	        if (null !== root) {
	          root.return = firstChild.return;
	          nextEffect = root;
	          break;
	        }
	        nextEffect = firstChild.return;
	      }
	}
	function commitLayoutEffectOnFiber(finishedRoot, current, finishedWork) {
	  var flags = finishedWork.flags;
	  switch (finishedWork.tag) {
	    case 0:
	    case 11:
	    case 15:
	      recursivelyTraverseLayoutEffects(finishedRoot, finishedWork);
	      flags & 4 && commitHookEffectListMount(5, finishedWork);
	      break;
	    case 1:
	      recursivelyTraverseLayoutEffects(finishedRoot, finishedWork);
	      if (flags & 4)
	        if (((finishedRoot = finishedWork.stateNode), null === current))
	          try {
	            finishedRoot.componentDidMount();
	          } catch (error) {
	            captureCommitPhaseError(finishedWork, finishedWork.return, error);
	          }
	        else {
	          var prevProps = resolveClassComponentProps(
	            finishedWork.type,
	            current.memoizedProps
	          );
	          current = current.memoizedState;
	          try {
	            finishedRoot.componentDidUpdate(
	              prevProps,
	              current,
	              finishedRoot.__reactInternalSnapshotBeforeUpdate
	            );
	          } catch (error$139) {
	            captureCommitPhaseError(
	              finishedWork,
	              finishedWork.return,
	              error$139
	            );
	          }
	        }
	      flags & 64 && commitClassCallbacks(finishedWork);
	      flags & 512 && safelyAttachRef(finishedWork, finishedWork.return);
	      break;
	    case 3:
	      recursivelyTraverseLayoutEffects(finishedRoot, finishedWork);
	      if (
	        flags & 64 &&
	        ((finishedRoot = finishedWork.updateQueue), null !== finishedRoot)
	      ) {
	        current = null;
	        if (null !== finishedWork.child)
	          switch (finishedWork.child.tag) {
	            case 27:
	            case 5:
	              current = finishedWork.child.stateNode;
	              break;
	            case 1:
	              current = finishedWork.child.stateNode;
	          }
	        try {
	          commitCallbacks(finishedRoot, current);
	        } catch (error) {
	          captureCommitPhaseError(finishedWork, finishedWork.return, error);
	        }
	      }
	      break;
	    case 27:
	      null === current &&
	        flags & 4 &&
	        commitHostSingletonAcquisition(finishedWork);
	    case 26:
	    case 5:
	      recursivelyTraverseLayoutEffects(finishedRoot, finishedWork);
	      null === current && flags & 4 && commitHostMount(finishedWork);
	      flags & 512 && safelyAttachRef(finishedWork, finishedWork.return);
	      break;
	    case 12:
	      recursivelyTraverseLayoutEffects(finishedRoot, finishedWork);
	      break;
	    case 31:
	      recursivelyTraverseLayoutEffects(finishedRoot, finishedWork);
	      flags & 4 && commitActivityHydrationCallbacks(finishedRoot, finishedWork);
	      break;
	    case 13:
	      recursivelyTraverseLayoutEffects(finishedRoot, finishedWork);
	      flags & 4 && commitSuspenseHydrationCallbacks(finishedRoot, finishedWork);
	      flags & 64 &&
	        ((finishedRoot = finishedWork.memoizedState),
	        null !== finishedRoot &&
	          ((finishedRoot = finishedRoot.dehydrated),
	          null !== finishedRoot &&
	            ((finishedWork = retryDehydratedSuspenseBoundary.bind(
	              null,
	              finishedWork
	            )),
	            registerSuspenseInstanceRetry(finishedRoot, finishedWork))));
	      break;
	    case 22:
	      flags = null !== finishedWork.memoizedState || offscreenSubtreeIsHidden;
	      if (!flags) {
	        current =
	          (null !== current && null !== current.memoizedState) ||
	          offscreenSubtreeWasHidden;
	        prevProps = offscreenSubtreeIsHidden;
	        var prevOffscreenSubtreeWasHidden = offscreenSubtreeWasHidden;
	        offscreenSubtreeIsHidden = flags;
	        (offscreenSubtreeWasHidden = current) && !prevOffscreenSubtreeWasHidden
	          ? recursivelyTraverseReappearLayoutEffects(
	              finishedRoot,
	              finishedWork,
	              0 !== (finishedWork.subtreeFlags & 8772)
	            )
	          : recursivelyTraverseLayoutEffects(finishedRoot, finishedWork);
	        offscreenSubtreeIsHidden = prevProps;
	        offscreenSubtreeWasHidden = prevOffscreenSubtreeWasHidden;
	      }
	      break;
	    case 30:
	      break;
	    default:
	      recursivelyTraverseLayoutEffects(finishedRoot, finishedWork);
	  }
	}
	function detachFiberAfterEffects(fiber) {
	  var alternate = fiber.alternate;
	  null !== alternate &&
	    ((fiber.alternate = null), detachFiberAfterEffects(alternate));
	  fiber.child = null;
	  fiber.deletions = null;
	  fiber.sibling = null;
	  5 === fiber.tag &&
	    ((alternate = fiber.stateNode),
	    null !== alternate && detachDeletedInstance(alternate));
	  fiber.stateNode = null;
	  fiber.return = null;
	  fiber.dependencies = null;
	  fiber.memoizedProps = null;
	  fiber.memoizedState = null;
	  fiber.pendingProps = null;
	  fiber.stateNode = null;
	  fiber.updateQueue = null;
	}
	var hostParent = null,
	  hostParentIsContainer = !1;
	function recursivelyTraverseDeletionEffects(
	  finishedRoot,
	  nearestMountedAncestor,
	  parent
	) {
	  for (parent = parent.child; null !== parent; )
	    commitDeletionEffectsOnFiber(finishedRoot, nearestMountedAncestor, parent),
	      (parent = parent.sibling);
	}
	function commitDeletionEffectsOnFiber(
	  finishedRoot,
	  nearestMountedAncestor,
	  deletedFiber
	) {
	  if (injectedHook && "function" === typeof injectedHook.onCommitFiberUnmount)
	    try {
	      injectedHook.onCommitFiberUnmount(rendererID, deletedFiber);
	    } catch (err) {}
	  switch (deletedFiber.tag) {
	    case 26:
	      offscreenSubtreeWasHidden ||
	        safelyDetachRef(deletedFiber, nearestMountedAncestor);
	      recursivelyTraverseDeletionEffects(
	        finishedRoot,
	        nearestMountedAncestor,
	        deletedFiber
	      );
	      deletedFiber.memoizedState
	        ? deletedFiber.memoizedState.count--
	        : deletedFiber.stateNode &&
	          ((deletedFiber = deletedFiber.stateNode),
	          deletedFiber.parentNode.removeChild(deletedFiber));
	      break;
	    case 27:
	      offscreenSubtreeWasHidden ||
	        safelyDetachRef(deletedFiber, nearestMountedAncestor);
	      var prevHostParent = hostParent,
	        prevHostParentIsContainer = hostParentIsContainer;
	      isSingletonScope(deletedFiber.type) &&
	        ((hostParent = deletedFiber.stateNode), (hostParentIsContainer = !1));
	      recursivelyTraverseDeletionEffects(
	        finishedRoot,
	        nearestMountedAncestor,
	        deletedFiber
	      );
	      releaseSingletonInstance(deletedFiber.stateNode);
	      hostParent = prevHostParent;
	      hostParentIsContainer = prevHostParentIsContainer;
	      break;
	    case 5:
	      offscreenSubtreeWasHidden ||
	        safelyDetachRef(deletedFiber, nearestMountedAncestor);
	    case 6:
	      prevHostParent = hostParent;
	      prevHostParentIsContainer = hostParentIsContainer;
	      hostParent = null;
	      recursivelyTraverseDeletionEffects(
	        finishedRoot,
	        nearestMountedAncestor,
	        deletedFiber
	      );
	      hostParent = prevHostParent;
	      hostParentIsContainer = prevHostParentIsContainer;
	      if (null !== hostParent)
	        if (hostParentIsContainer)
	          try {
	            (9 === hostParent.nodeType
	              ? hostParent.body
	              : "HTML" === hostParent.nodeName
	                ? hostParent.ownerDocument.body
	                : hostParent
	            ).removeChild(deletedFiber.stateNode);
	          } catch (error) {
	            captureCommitPhaseError(
	              deletedFiber,
	              nearestMountedAncestor,
	              error
	            );
	          }
	        else
	          try {
	            hostParent.removeChild(deletedFiber.stateNode);
	          } catch (error) {
	            captureCommitPhaseError(
	              deletedFiber,
	              nearestMountedAncestor,
	              error
	            );
	          }
	      break;
	    case 18:
	      null !== hostParent &&
	        (hostParentIsContainer
	          ? ((finishedRoot = hostParent),
	            clearHydrationBoundary(
	              9 === finishedRoot.nodeType
	                ? finishedRoot.body
	                : "HTML" === finishedRoot.nodeName
	                  ? finishedRoot.ownerDocument.body
	                  : finishedRoot,
	              deletedFiber.stateNode
	            ),
	            retryIfBlockedOn(finishedRoot))
	          : clearHydrationBoundary(hostParent, deletedFiber.stateNode));
	      break;
	    case 4:
	      prevHostParent = hostParent;
	      prevHostParentIsContainer = hostParentIsContainer;
	      hostParent = deletedFiber.stateNode.containerInfo;
	      hostParentIsContainer = !0;
	      recursivelyTraverseDeletionEffects(
	        finishedRoot,
	        nearestMountedAncestor,
	        deletedFiber
	      );
	      hostParent = prevHostParent;
	      hostParentIsContainer = prevHostParentIsContainer;
	      break;
	    case 0:
	    case 11:
	    case 14:
	    case 15:
	      commitHookEffectListUnmount(2, deletedFiber, nearestMountedAncestor);
	      offscreenSubtreeWasHidden ||
	        commitHookEffectListUnmount(4, deletedFiber, nearestMountedAncestor);
	      recursivelyTraverseDeletionEffects(
	        finishedRoot,
	        nearestMountedAncestor,
	        deletedFiber
	      );
	      break;
	    case 1:
	      offscreenSubtreeWasHidden ||
	        (safelyDetachRef(deletedFiber, nearestMountedAncestor),
	        (prevHostParent = deletedFiber.stateNode),
	        "function" === typeof prevHostParent.componentWillUnmount &&
	          safelyCallComponentWillUnmount(
	            deletedFiber,
	            nearestMountedAncestor,
	            prevHostParent
	          ));
	      recursivelyTraverseDeletionEffects(
	        finishedRoot,
	        nearestMountedAncestor,
	        deletedFiber
	      );
	      break;
	    case 21:
	      recursivelyTraverseDeletionEffects(
	        finishedRoot,
	        nearestMountedAncestor,
	        deletedFiber
	      );
	      break;
	    case 22:
	      offscreenSubtreeWasHidden =
	        (prevHostParent = offscreenSubtreeWasHidden) ||
	        null !== deletedFiber.memoizedState;
	      recursivelyTraverseDeletionEffects(
	        finishedRoot,
	        nearestMountedAncestor,
	        deletedFiber
	      );
	      offscreenSubtreeWasHidden = prevHostParent;
	      break;
	    default:
	      recursivelyTraverseDeletionEffects(
	        finishedRoot,
	        nearestMountedAncestor,
	        deletedFiber
	      );
	  }
	}
	function commitActivityHydrationCallbacks(finishedRoot, finishedWork) {
	  if (
	    null === finishedWork.memoizedState &&
	    ((finishedRoot = finishedWork.alternate),
	    null !== finishedRoot &&
	      ((finishedRoot = finishedRoot.memoizedState), null !== finishedRoot))
	  ) {
	    finishedRoot = finishedRoot.dehydrated;
	    try {
	      retryIfBlockedOn(finishedRoot);
	    } catch (error) {
	      captureCommitPhaseError(finishedWork, finishedWork.return, error);
	    }
	  }
	}
	function commitSuspenseHydrationCallbacks(finishedRoot, finishedWork) {
	  if (
	    null === finishedWork.memoizedState &&
	    ((finishedRoot = finishedWork.alternate),
	    null !== finishedRoot &&
	      ((finishedRoot = finishedRoot.memoizedState),
	      null !== finishedRoot &&
	        ((finishedRoot = finishedRoot.dehydrated), null !== finishedRoot)))
	  )
	    try {
	      retryIfBlockedOn(finishedRoot);
	    } catch (error) {
	      captureCommitPhaseError(finishedWork, finishedWork.return, error);
	    }
	}
	function getRetryCache(finishedWork) {
	  switch (finishedWork.tag) {
	    case 31:
	    case 13:
	    case 19:
	      var retryCache = finishedWork.stateNode;
	      null === retryCache &&
	        (retryCache = finishedWork.stateNode = new PossiblyWeakSet());
	      return retryCache;
	    case 22:
	      return (
	        (finishedWork = finishedWork.stateNode),
	        (retryCache = finishedWork._retryCache),
	        null === retryCache &&
	          (retryCache = finishedWork._retryCache = new PossiblyWeakSet()),
	        retryCache
	      );
	    default:
	      throw Error(formatProdErrorMessage(435, finishedWork.tag));
	  }
	}
	function attachSuspenseRetryListeners(finishedWork, wakeables) {
	  var retryCache = getRetryCache(finishedWork);
	  wakeables.forEach(function (wakeable) {
	    if (!retryCache.has(wakeable)) {
	      retryCache.add(wakeable);
	      var retry = resolveRetryWakeable.bind(null, finishedWork, wakeable);
	      wakeable.then(retry, retry);
	    }
	  });
	}
	function recursivelyTraverseMutationEffects(root$jscomp$0, parentFiber) {
	  var deletions = parentFiber.deletions;
	  if (null !== deletions)
	    for (var i = 0; i < deletions.length; i++) {
	      var childToDelete = deletions[i],
	        root = root$jscomp$0,
	        returnFiber = parentFiber,
	        parent = returnFiber;
	      a: for (; null !== parent; ) {
	        switch (parent.tag) {
	          case 27:
	            if (isSingletonScope(parent.type)) {
	              hostParent = parent.stateNode;
	              hostParentIsContainer = !1;
	              break a;
	            }
	            break;
	          case 5:
	            hostParent = parent.stateNode;
	            hostParentIsContainer = !1;
	            break a;
	          case 3:
	          case 4:
	            hostParent = parent.stateNode.containerInfo;
	            hostParentIsContainer = !0;
	            break a;
	        }
	        parent = parent.return;
	      }
	      if (null === hostParent) throw Error(formatProdErrorMessage(160));
	      commitDeletionEffectsOnFiber(root, returnFiber, childToDelete);
	      hostParent = null;
	      hostParentIsContainer = !1;
	      root = childToDelete.alternate;
	      null !== root && (root.return = null);
	      childToDelete.return = null;
	    }
	  if (parentFiber.subtreeFlags & 13886)
	    for (parentFiber = parentFiber.child; null !== parentFiber; )
	      commitMutationEffectsOnFiber(parentFiber, root$jscomp$0),
	        (parentFiber = parentFiber.sibling);
	}
	var currentHoistableRoot = null;
	function commitMutationEffectsOnFiber(finishedWork, root) {
	  var current = finishedWork.alternate,
	    flags = finishedWork.flags;
	  switch (finishedWork.tag) {
	    case 0:
	    case 11:
	    case 14:
	    case 15:
	      recursivelyTraverseMutationEffects(root, finishedWork);
	      commitReconciliationEffects(finishedWork);
	      flags & 4 &&
	        (commitHookEffectListUnmount(3, finishedWork, finishedWork.return),
	        commitHookEffectListMount(3, finishedWork),
	        commitHookEffectListUnmount(5, finishedWork, finishedWork.return));
	      break;
	    case 1:
	      recursivelyTraverseMutationEffects(root, finishedWork);
	      commitReconciliationEffects(finishedWork);
	      flags & 512 &&
	        (offscreenSubtreeWasHidden ||
	          null === current ||
	          safelyDetachRef(current, current.return));
	      flags & 64 &&
	        offscreenSubtreeIsHidden &&
	        ((finishedWork = finishedWork.updateQueue),
	        null !== finishedWork &&
	          ((flags = finishedWork.callbacks),
	          null !== flags &&
	            ((current = finishedWork.shared.hiddenCallbacks),
	            (finishedWork.shared.hiddenCallbacks =
	              null === current ? flags : current.concat(flags)))));
	      break;
	    case 26:
	      var hoistableRoot = currentHoistableRoot;
	      recursivelyTraverseMutationEffects(root, finishedWork);
	      commitReconciliationEffects(finishedWork);
	      flags & 512 &&
	        (offscreenSubtreeWasHidden ||
	          null === current ||
	          safelyDetachRef(current, current.return));
	      if (flags & 4) {
	        var currentResource = null !== current ? current.memoizedState : null;
	        flags = finishedWork.memoizedState;
	        if (null === current)
	          if (null === flags)
	            if (null === finishedWork.stateNode) {
	              a: {
	                flags = finishedWork.type;
	                current = finishedWork.memoizedProps;
	                hoistableRoot = hoistableRoot.ownerDocument || hoistableRoot;
	                b: switch (flags) {
	                  case "title":
	                    currentResource =
	                      hoistableRoot.getElementsByTagName("title")[0];
	                    if (
	                      !currentResource ||
	                      currentResource[internalHoistableMarker] ||
	                      currentResource[internalInstanceKey] ||
	                      "http://www.w3.org/2000/svg" ===
	                        currentResource.namespaceURI ||
	                      currentResource.hasAttribute("itemprop")
	                    )
	                      (currentResource = hoistableRoot.createElement(flags)),
	                        hoistableRoot.head.insertBefore(
	                          currentResource,
	                          hoistableRoot.querySelector("head > title")
	                        );
	                    setInitialProperties(currentResource, flags, current);
	                    currentResource[internalInstanceKey] = finishedWork;
	                    markNodeAsHoistable(currentResource);
	                    flags = currentResource;
	                    break a;
	                  case "link":
	                    var maybeNodes = getHydratableHoistableCache(
	                      "link",
	                      "href",
	                      hoistableRoot
	                    ).get(flags + (current.href || ""));
	                    if (maybeNodes)
	                      for (var i = 0; i < maybeNodes.length; i++)
	                        if (
	                          ((currentResource = maybeNodes[i]),
	                          currentResource.getAttribute("href") ===
	                            (null == current.href || "" === current.href
	                              ? null
	                              : current.href) &&
	                            currentResource.getAttribute("rel") ===
	                              (null == current.rel ? null : current.rel) &&
	                            currentResource.getAttribute("title") ===
	                              (null == current.title ? null : current.title) &&
	                            currentResource.getAttribute("crossorigin") ===
	                              (null == current.crossOrigin
	                                ? null
	                                : current.crossOrigin))
	                        ) {
	                          maybeNodes.splice(i, 1);
	                          break b;
	                        }
	                    currentResource = hoistableRoot.createElement(flags);
	                    setInitialProperties(currentResource, flags, current);
	                    hoistableRoot.head.appendChild(currentResource);
	                    break;
	                  case "meta":
	                    if (
	                      (maybeNodes = getHydratableHoistableCache(
	                        "meta",
	                        "content",
	                        hoistableRoot
	                      ).get(flags + (current.content || "")))
	                    )
	                      for (i = 0; i < maybeNodes.length; i++)
	                        if (
	                          ((currentResource = maybeNodes[i]),
	                          currentResource.getAttribute("content") ===
	                            (null == current.content
	                              ? null
	                              : "" + current.content) &&
	                            currentResource.getAttribute("name") ===
	                              (null == current.name ? null : current.name) &&
	                            currentResource.getAttribute("property") ===
	                              (null == current.property
	                                ? null
	                                : current.property) &&
	                            currentResource.getAttribute("http-equiv") ===
	                              (null == current.httpEquiv
	                                ? null
	                                : current.httpEquiv) &&
	                            currentResource.getAttribute("charset") ===
	                              (null == current.charSet
	                                ? null
	                                : current.charSet))
	                        ) {
	                          maybeNodes.splice(i, 1);
	                          break b;
	                        }
	                    currentResource = hoistableRoot.createElement(flags);
	                    setInitialProperties(currentResource, flags, current);
	                    hoistableRoot.head.appendChild(currentResource);
	                    break;
	                  default:
	                    throw Error(formatProdErrorMessage(468, flags));
	                }
	                currentResource[internalInstanceKey] = finishedWork;
	                markNodeAsHoistable(currentResource);
	                flags = currentResource;
	              }
	              finishedWork.stateNode = flags;
	            } else
	              mountHoistable(
	                hoistableRoot,
	                finishedWork.type,
	                finishedWork.stateNode
	              );
	          else
	            finishedWork.stateNode = acquireResource(
	              hoistableRoot,
	              flags,
	              finishedWork.memoizedProps
	            );
	        else
	          currentResource !== flags
	            ? (null === currentResource
	                ? null !== current.stateNode &&
	                  ((current = current.stateNode),
	                  current.parentNode.removeChild(current))
	                : currentResource.count--,
	              null === flags
	                ? mountHoistable(
	                    hoistableRoot,
	                    finishedWork.type,
	                    finishedWork.stateNode
	                  )
	                : acquireResource(
	                    hoistableRoot,
	                    flags,
	                    finishedWork.memoizedProps
	                  ))
	            : null === flags &&
	              null !== finishedWork.stateNode &&
	              commitHostUpdate(
	                finishedWork,
	                finishedWork.memoizedProps,
	                current.memoizedProps
	              );
	      }
	      break;
	    case 27:
	      recursivelyTraverseMutationEffects(root, finishedWork);
	      commitReconciliationEffects(finishedWork);
	      flags & 512 &&
	        (offscreenSubtreeWasHidden ||
	          null === current ||
	          safelyDetachRef(current, current.return));
	      null !== current &&
	        flags & 4 &&
	        commitHostUpdate(
	          finishedWork,
	          finishedWork.memoizedProps,
	          current.memoizedProps
	        );
	      break;
	    case 5:
	      recursivelyTraverseMutationEffects(root, finishedWork);
	      commitReconciliationEffects(finishedWork);
	      flags & 512 &&
	        (offscreenSubtreeWasHidden ||
	          null === current ||
	          safelyDetachRef(current, current.return));
	      if (finishedWork.flags & 32) {
	        hoistableRoot = finishedWork.stateNode;
	        try {
	          setTextContent(hoistableRoot, "");
	        } catch (error) {
	          captureCommitPhaseError(finishedWork, finishedWork.return, error);
	        }
	      }
	      flags & 4 &&
	        null != finishedWork.stateNode &&
	        ((hoistableRoot = finishedWork.memoizedProps),
	        commitHostUpdate(
	          finishedWork,
	          hoistableRoot,
	          null !== current ? current.memoizedProps : hoistableRoot
	        ));
	      flags & 1024 && (needsFormReset = !0);
	      break;
	    case 6:
	      recursivelyTraverseMutationEffects(root, finishedWork);
	      commitReconciliationEffects(finishedWork);
	      if (flags & 4) {
	        if (null === finishedWork.stateNode)
	          throw Error(formatProdErrorMessage(162));
	        flags = finishedWork.memoizedProps;
	        current = finishedWork.stateNode;
	        try {
	          current.nodeValue = flags;
	        } catch (error) {
	          captureCommitPhaseError(finishedWork, finishedWork.return, error);
	        }
	      }
	      break;
	    case 3:
	      tagCaches = null;
	      hoistableRoot = currentHoistableRoot;
	      currentHoistableRoot = getHoistableRoot(root.containerInfo);
	      recursivelyTraverseMutationEffects(root, finishedWork);
	      currentHoistableRoot = hoistableRoot;
	      commitReconciliationEffects(finishedWork);
	      if (flags & 4 && null !== current && current.memoizedState.isDehydrated)
	        try {
	          retryIfBlockedOn(root.containerInfo);
	        } catch (error) {
	          captureCommitPhaseError(finishedWork, finishedWork.return, error);
	        }
	      needsFormReset &&
	        ((needsFormReset = !1), recursivelyResetForms(finishedWork));
	      break;
	    case 4:
	      flags = currentHoistableRoot;
	      currentHoistableRoot = getHoistableRoot(
	        finishedWork.stateNode.containerInfo
	      );
	      recursivelyTraverseMutationEffects(root, finishedWork);
	      commitReconciliationEffects(finishedWork);
	      currentHoistableRoot = flags;
	      break;
	    case 12:
	      recursivelyTraverseMutationEffects(root, finishedWork);
	      commitReconciliationEffects(finishedWork);
	      break;
	    case 31:
	      recursivelyTraverseMutationEffects(root, finishedWork);
	      commitReconciliationEffects(finishedWork);
	      flags & 4 &&
	        ((flags = finishedWork.updateQueue),
	        null !== flags &&
	          ((finishedWork.updateQueue = null),
	          attachSuspenseRetryListeners(finishedWork, flags)));
	      break;
	    case 13:
	      recursivelyTraverseMutationEffects(root, finishedWork);
	      commitReconciliationEffects(finishedWork);
	      finishedWork.child.flags & 8192 &&
	        (null !== finishedWork.memoizedState) !==
	          (null !== current && null !== current.memoizedState) &&
	        (globalMostRecentFallbackTime = now());
	      flags & 4 &&
	        ((flags = finishedWork.updateQueue),
	        null !== flags &&
	          ((finishedWork.updateQueue = null),
	          attachSuspenseRetryListeners(finishedWork, flags)));
	      break;
	    case 22:
	      hoistableRoot = null !== finishedWork.memoizedState;
	      var wasHidden = null !== current && null !== current.memoizedState,
	        prevOffscreenSubtreeIsHidden = offscreenSubtreeIsHidden,
	        prevOffscreenSubtreeWasHidden = offscreenSubtreeWasHidden;
	      offscreenSubtreeIsHidden = prevOffscreenSubtreeIsHidden || hoistableRoot;
	      offscreenSubtreeWasHidden = prevOffscreenSubtreeWasHidden || wasHidden;
	      recursivelyTraverseMutationEffects(root, finishedWork);
	      offscreenSubtreeWasHidden = prevOffscreenSubtreeWasHidden;
	      offscreenSubtreeIsHidden = prevOffscreenSubtreeIsHidden;
	      commitReconciliationEffects(finishedWork);
	      if (flags & 8192)
	        a: for (
	          root = finishedWork.stateNode,
	            root._visibility = hoistableRoot
	              ? root._visibility & -2
	              : root._visibility | 1,
	            hoistableRoot &&
	              (null === current ||
	                wasHidden ||
	                offscreenSubtreeIsHidden ||
	                offscreenSubtreeWasHidden ||
	                recursivelyTraverseDisappearLayoutEffects(finishedWork)),
	            current = null,
	            root = finishedWork;
	          ;

	        ) {
	          if (5 === root.tag || 26 === root.tag) {
	            if (null === current) {
	              wasHidden = current = root;
	              try {
	                if (((currentResource = wasHidden.stateNode), hoistableRoot))
	                  (maybeNodes = currentResource.style),
	                    "function" === typeof maybeNodes.setProperty
	                      ? maybeNodes.setProperty("display", "none", "important")
	                      : (maybeNodes.display = "none");
	                else {
	                  i = wasHidden.stateNode;
	                  var styleProp = wasHidden.memoizedProps.style,
	                    display =
	                      void 0 !== styleProp &&
	                      null !== styleProp &&
	                      styleProp.hasOwnProperty("display")
	                        ? styleProp.display
	                        : null;
	                  i.style.display =
	                    null == display || "boolean" === typeof display
	                      ? ""
	                      : ("" + display).trim();
	                }
	              } catch (error) {
	                captureCommitPhaseError(wasHidden, wasHidden.return, error);
	              }
	            }
	          } else if (6 === root.tag) {
	            if (null === current) {
	              wasHidden = root;
	              try {
	                wasHidden.stateNode.nodeValue = hoistableRoot
	                  ? ""
	                  : wasHidden.memoizedProps;
	              } catch (error) {
	                captureCommitPhaseError(wasHidden, wasHidden.return, error);
	              }
	            }
	          } else if (18 === root.tag) {
	            if (null === current) {
	              wasHidden = root;
	              try {
	                var instance = wasHidden.stateNode;
	                hoistableRoot
	                  ? hideOrUnhideDehydratedBoundary(instance, !0)
	                  : hideOrUnhideDehydratedBoundary(wasHidden.stateNode, !1);
	              } catch (error) {
	                captureCommitPhaseError(wasHidden, wasHidden.return, error);
	              }
	            }
	          } else if (
	            ((22 !== root.tag && 23 !== root.tag) ||
	              null === root.memoizedState ||
	              root === finishedWork) &&
	            null !== root.child
	          ) {
	            root.child.return = root;
	            root = root.child;
	            continue;
	          }
	          if (root === finishedWork) break a;
	          for (; null === root.sibling; ) {
	            if (null === root.return || root.return === finishedWork) break a;
	            current === root && (current = null);
	            root = root.return;
	          }
	          current === root && (current = null);
	          root.sibling.return = root.return;
	          root = root.sibling;
	        }
	      flags & 4 &&
	        ((flags = finishedWork.updateQueue),
	        null !== flags &&
	          ((current = flags.retryQueue),
	          null !== current &&
	            ((flags.retryQueue = null),
	            attachSuspenseRetryListeners(finishedWork, current))));
	      break;
	    case 19:
	      recursivelyTraverseMutationEffects(root, finishedWork);
	      commitReconciliationEffects(finishedWork);
	      flags & 4 &&
	        ((flags = finishedWork.updateQueue),
	        null !== flags &&
	          ((finishedWork.updateQueue = null),
	          attachSuspenseRetryListeners(finishedWork, flags)));
	      break;
	    case 30:
	      break;
	    case 21:
	      break;
	    default:
	      recursivelyTraverseMutationEffects(root, finishedWork),
	        commitReconciliationEffects(finishedWork);
	  }
	}
	function commitReconciliationEffects(finishedWork) {
	  var flags = finishedWork.flags;
	  if (flags & 2) {
	    try {
	      for (
	        var hostParentFiber, parentFiber = finishedWork.return;
	        null !== parentFiber;

	      ) {
	        if (isHostParent(parentFiber)) {
	          hostParentFiber = parentFiber;
	          break;
	        }
	        parentFiber = parentFiber.return;
	      }
	      if (null == hostParentFiber) throw Error(formatProdErrorMessage(160));
	      switch (hostParentFiber.tag) {
	        case 27:
	          var parent = hostParentFiber.stateNode,
	            before = getHostSibling(finishedWork);
	          insertOrAppendPlacementNode(finishedWork, before, parent);
	          break;
	        case 5:
	          var parent$141 = hostParentFiber.stateNode;
	          hostParentFiber.flags & 32 &&
	            (setTextContent(parent$141, ""), (hostParentFiber.flags &= -33));
	          var before$142 = getHostSibling(finishedWork);
	          insertOrAppendPlacementNode(finishedWork, before$142, parent$141);
	          break;
	        case 3:
	        case 4:
	          var parent$143 = hostParentFiber.stateNode.containerInfo,
	            before$144 = getHostSibling(finishedWork);
	          insertOrAppendPlacementNodeIntoContainer(
	            finishedWork,
	            before$144,
	            parent$143
	          );
	          break;
	        default:
	          throw Error(formatProdErrorMessage(161));
	      }
	    } catch (error) {
	      captureCommitPhaseError(finishedWork, finishedWork.return, error);
	    }
	    finishedWork.flags &= -3;
	  }
	  flags & 4096 && (finishedWork.flags &= -4097);
	}
	function recursivelyResetForms(parentFiber) {
	  if (parentFiber.subtreeFlags & 1024)
	    for (parentFiber = parentFiber.child; null !== parentFiber; ) {
	      var fiber = parentFiber;
	      recursivelyResetForms(fiber);
	      5 === fiber.tag && fiber.flags & 1024 && fiber.stateNode.reset();
	      parentFiber = parentFiber.sibling;
	    }
	}
	function recursivelyTraverseLayoutEffects(root, parentFiber) {
	  if (parentFiber.subtreeFlags & 8772)
	    for (parentFiber = parentFiber.child; null !== parentFiber; )
	      commitLayoutEffectOnFiber(root, parentFiber.alternate, parentFiber),
	        (parentFiber = parentFiber.sibling);
	}
	function recursivelyTraverseDisappearLayoutEffects(parentFiber) {
	  for (parentFiber = parentFiber.child; null !== parentFiber; ) {
	    var finishedWork = parentFiber;
	    switch (finishedWork.tag) {
	      case 0:
	      case 11:
	      case 14:
	      case 15:
	        commitHookEffectListUnmount(4, finishedWork, finishedWork.return);
	        recursivelyTraverseDisappearLayoutEffects(finishedWork);
	        break;
	      case 1:
	        safelyDetachRef(finishedWork, finishedWork.return);
	        var instance = finishedWork.stateNode;
	        "function" === typeof instance.componentWillUnmount &&
	          safelyCallComponentWillUnmount(
	            finishedWork,
	            finishedWork.return,
	            instance
	          );
	        recursivelyTraverseDisappearLayoutEffects(finishedWork);
	        break;
	      case 27:
	        releaseSingletonInstance(finishedWork.stateNode);
	      case 26:
	      case 5:
	        safelyDetachRef(finishedWork, finishedWork.return);
	        recursivelyTraverseDisappearLayoutEffects(finishedWork);
	        break;
	      case 22:
	        null === finishedWork.memoizedState &&
	          recursivelyTraverseDisappearLayoutEffects(finishedWork);
	        break;
	      case 30:
	        recursivelyTraverseDisappearLayoutEffects(finishedWork);
	        break;
	      default:
	        recursivelyTraverseDisappearLayoutEffects(finishedWork);
	    }
	    parentFiber = parentFiber.sibling;
	  }
	}
	function recursivelyTraverseReappearLayoutEffects(
	  finishedRoot$jscomp$0,
	  parentFiber,
	  includeWorkInProgressEffects
	) {
	  includeWorkInProgressEffects =
	    includeWorkInProgressEffects && 0 !== (parentFiber.subtreeFlags & 8772);
	  for (parentFiber = parentFiber.child; null !== parentFiber; ) {
	    var current = parentFiber.alternate,
	      finishedRoot = finishedRoot$jscomp$0,
	      finishedWork = parentFiber,
	      flags = finishedWork.flags;
	    switch (finishedWork.tag) {
	      case 0:
	      case 11:
	      case 15:
	        recursivelyTraverseReappearLayoutEffects(
	          finishedRoot,
	          finishedWork,
	          includeWorkInProgressEffects
	        );
	        commitHookEffectListMount(4, finishedWork);
	        break;
	      case 1:
	        recursivelyTraverseReappearLayoutEffects(
	          finishedRoot,
	          finishedWork,
	          includeWorkInProgressEffects
	        );
	        current = finishedWork;
	        finishedRoot = current.stateNode;
	        if ("function" === typeof finishedRoot.componentDidMount)
	          try {
	            finishedRoot.componentDidMount();
	          } catch (error) {
	            captureCommitPhaseError(current, current.return, error);
	          }
	        current = finishedWork;
	        finishedRoot = current.updateQueue;
	        if (null !== finishedRoot) {
	          var instance = current.stateNode;
	          try {
	            var hiddenCallbacks = finishedRoot.shared.hiddenCallbacks;
	            if (null !== hiddenCallbacks)
	              for (
	                finishedRoot.shared.hiddenCallbacks = null, finishedRoot = 0;
	                finishedRoot < hiddenCallbacks.length;
	                finishedRoot++
	              )
	                callCallback(hiddenCallbacks[finishedRoot], instance);
	          } catch (error) {
	            captureCommitPhaseError(current, current.return, error);
	          }
	        }
	        includeWorkInProgressEffects &&
	          flags & 64 &&
	          commitClassCallbacks(finishedWork);
	        safelyAttachRef(finishedWork, finishedWork.return);
	        break;
	      case 27:
	        commitHostSingletonAcquisition(finishedWork);
	      case 26:
	      case 5:
	        recursivelyTraverseReappearLayoutEffects(
	          finishedRoot,
	          finishedWork,
	          includeWorkInProgressEffects
	        );
	        includeWorkInProgressEffects &&
	          null === current &&
	          flags & 4 &&
	          commitHostMount(finishedWork);
	        safelyAttachRef(finishedWork, finishedWork.return);
	        break;
	      case 12:
	        recursivelyTraverseReappearLayoutEffects(
	          finishedRoot,
	          finishedWork,
	          includeWorkInProgressEffects
	        );
	        break;
	      case 31:
	        recursivelyTraverseReappearLayoutEffects(
	          finishedRoot,
	          finishedWork,
	          includeWorkInProgressEffects
	        );
	        includeWorkInProgressEffects &&
	          flags & 4 &&
	          commitActivityHydrationCallbacks(finishedRoot, finishedWork);
	        break;
	      case 13:
	        recursivelyTraverseReappearLayoutEffects(
	          finishedRoot,
	          finishedWork,
	          includeWorkInProgressEffects
	        );
	        includeWorkInProgressEffects &&
	          flags & 4 &&
	          commitSuspenseHydrationCallbacks(finishedRoot, finishedWork);
	        break;
	      case 22:
	        null === finishedWork.memoizedState &&
	          recursivelyTraverseReappearLayoutEffects(
	            finishedRoot,
	            finishedWork,
	            includeWorkInProgressEffects
	          );
	        safelyAttachRef(finishedWork, finishedWork.return);
	        break;
	      case 30:
	        break;
	      default:
	        recursivelyTraverseReappearLayoutEffects(
	          finishedRoot,
	          finishedWork,
	          includeWorkInProgressEffects
	        );
	    }
	    parentFiber = parentFiber.sibling;
	  }
	}
	function commitOffscreenPassiveMountEffects(current, finishedWork) {
	  var previousCache = null;
	  null !== current &&
	    null !== current.memoizedState &&
	    null !== current.memoizedState.cachePool &&
	    (previousCache = current.memoizedState.cachePool.pool);
	  current = null;
	  null !== finishedWork.memoizedState &&
	    null !== finishedWork.memoizedState.cachePool &&
	    (current = finishedWork.memoizedState.cachePool.pool);
	  current !== previousCache &&
	    (null != current && current.refCount++,
	    null != previousCache && releaseCache(previousCache));
	}
	function commitCachePassiveMountEffect(current, finishedWork) {
	  current = null;
	  null !== finishedWork.alternate &&
	    (current = finishedWork.alternate.memoizedState.cache);
	  finishedWork = finishedWork.memoizedState.cache;
	  finishedWork !== current &&
	    (finishedWork.refCount++, null != current && releaseCache(current));
	}
	function recursivelyTraversePassiveMountEffects(
	  root,
	  parentFiber,
	  committedLanes,
	  committedTransitions
	) {
	  if (parentFiber.subtreeFlags & 10256)
	    for (parentFiber = parentFiber.child; null !== parentFiber; )
	      commitPassiveMountOnFiber(
	        root,
	        parentFiber,
	        committedLanes,
	        committedTransitions
	      ),
	        (parentFiber = parentFiber.sibling);
	}
	function commitPassiveMountOnFiber(
	  finishedRoot,
	  finishedWork,
	  committedLanes,
	  committedTransitions
	) {
	  var flags = finishedWork.flags;
	  switch (finishedWork.tag) {
	    case 0:
	    case 11:
	    case 15:
	      recursivelyTraversePassiveMountEffects(
	        finishedRoot,
	        finishedWork,
	        committedLanes,
	        committedTransitions
	      );
	      flags & 2048 && commitHookEffectListMount(9, finishedWork);
	      break;
	    case 1:
	      recursivelyTraversePassiveMountEffects(
	        finishedRoot,
	        finishedWork,
	        committedLanes,
	        committedTransitions
	      );
	      break;
	    case 3:
	      recursivelyTraversePassiveMountEffects(
	        finishedRoot,
	        finishedWork,
	        committedLanes,
	        committedTransitions
	      );
	      flags & 2048 &&
	        ((finishedRoot = null),
	        null !== finishedWork.alternate &&
	          (finishedRoot = finishedWork.alternate.memoizedState.cache),
	        (finishedWork = finishedWork.memoizedState.cache),
	        finishedWork !== finishedRoot &&
	          (finishedWork.refCount++,
	          null != finishedRoot && releaseCache(finishedRoot)));
	      break;
	    case 12:
	      if (flags & 2048) {
	        recursivelyTraversePassiveMountEffects(
	          finishedRoot,
	          finishedWork,
	          committedLanes,
	          committedTransitions
	        );
	        finishedRoot = finishedWork.stateNode;
	        try {
	          var _finishedWork$memoize2 = finishedWork.memoizedProps,
	            id = _finishedWork$memoize2.id,
	            onPostCommit = _finishedWork$memoize2.onPostCommit;
	          "function" === typeof onPostCommit &&
	            onPostCommit(
	              id,
	              null === finishedWork.alternate ? "mount" : "update",
	              finishedRoot.passiveEffectDuration,
	              -0
	            );
	        } catch (error) {
	          captureCommitPhaseError(finishedWork, finishedWork.return, error);
	        }
	      } else
	        recursivelyTraversePassiveMountEffects(
	          finishedRoot,
	          finishedWork,
	          committedLanes,
	          committedTransitions
	        );
	      break;
	    case 31:
	      recursivelyTraversePassiveMountEffects(
	        finishedRoot,
	        finishedWork,
	        committedLanes,
	        committedTransitions
	      );
	      break;
	    case 13:
	      recursivelyTraversePassiveMountEffects(
	        finishedRoot,
	        finishedWork,
	        committedLanes,
	        committedTransitions
	      );
	      break;
	    case 23:
	      break;
	    case 22:
	      _finishedWork$memoize2 = finishedWork.stateNode;
	      id = finishedWork.alternate;
	      null !== finishedWork.memoizedState
	        ? _finishedWork$memoize2._visibility & 2
	          ? recursivelyTraversePassiveMountEffects(
	              finishedRoot,
	              finishedWork,
	              committedLanes,
	              committedTransitions
	            )
	          : recursivelyTraverseAtomicPassiveEffects(finishedRoot, finishedWork)
	        : _finishedWork$memoize2._visibility & 2
	          ? recursivelyTraversePassiveMountEffects(
	              finishedRoot,
	              finishedWork,
	              committedLanes,
	              committedTransitions
	            )
	          : ((_finishedWork$memoize2._visibility |= 2),
	            recursivelyTraverseReconnectPassiveEffects(
	              finishedRoot,
	              finishedWork,
	              committedLanes,
	              committedTransitions,
	              0 !== (finishedWork.subtreeFlags & 10256) || !1
	            ));
	      flags & 2048 && commitOffscreenPassiveMountEffects(id, finishedWork);
	      break;
	    case 24:
	      recursivelyTraversePassiveMountEffects(
	        finishedRoot,
	        finishedWork,
	        committedLanes,
	        committedTransitions
	      );
	      flags & 2048 &&
	        commitCachePassiveMountEffect(finishedWork.alternate, finishedWork);
	      break;
	    default:
	      recursivelyTraversePassiveMountEffects(
	        finishedRoot,
	        finishedWork,
	        committedLanes,
	        committedTransitions
	      );
	  }
	}
	function recursivelyTraverseReconnectPassiveEffects(
	  finishedRoot$jscomp$0,
	  parentFiber,
	  committedLanes$jscomp$0,
	  committedTransitions$jscomp$0,
	  includeWorkInProgressEffects
	) {
	  includeWorkInProgressEffects =
	    includeWorkInProgressEffects &&
	    (0 !== (parentFiber.subtreeFlags & 10256) || !1);
	  for (parentFiber = parentFiber.child; null !== parentFiber; ) {
	    var finishedRoot = finishedRoot$jscomp$0,
	      finishedWork = parentFiber,
	      committedLanes = committedLanes$jscomp$0,
	      committedTransitions = committedTransitions$jscomp$0,
	      flags = finishedWork.flags;
	    switch (finishedWork.tag) {
	      case 0:
	      case 11:
	      case 15:
	        recursivelyTraverseReconnectPassiveEffects(
	          finishedRoot,
	          finishedWork,
	          committedLanes,
	          committedTransitions,
	          includeWorkInProgressEffects
	        );
	        commitHookEffectListMount(8, finishedWork);
	        break;
	      case 23:
	        break;
	      case 22:
	        var instance = finishedWork.stateNode;
	        null !== finishedWork.memoizedState
	          ? instance._visibility & 2
	            ? recursivelyTraverseReconnectPassiveEffects(
	                finishedRoot,
	                finishedWork,
	                committedLanes,
	                committedTransitions,
	                includeWorkInProgressEffects
	              )
	            : recursivelyTraverseAtomicPassiveEffects(
	                finishedRoot,
	                finishedWork
	              )
	          : ((instance._visibility |= 2),
	            recursivelyTraverseReconnectPassiveEffects(
	              finishedRoot,
	              finishedWork,
	              committedLanes,
	              committedTransitions,
	              includeWorkInProgressEffects
	            ));
	        includeWorkInProgressEffects &&
	          flags & 2048 &&
	          commitOffscreenPassiveMountEffects(
	            finishedWork.alternate,
	            finishedWork
	          );
	        break;
	      case 24:
	        recursivelyTraverseReconnectPassiveEffects(
	          finishedRoot,
	          finishedWork,
	          committedLanes,
	          committedTransitions,
	          includeWorkInProgressEffects
	        );
	        includeWorkInProgressEffects &&
	          flags & 2048 &&
	          commitCachePassiveMountEffect(finishedWork.alternate, finishedWork);
	        break;
	      default:
	        recursivelyTraverseReconnectPassiveEffects(
	          finishedRoot,
	          finishedWork,
	          committedLanes,
	          committedTransitions,
	          includeWorkInProgressEffects
	        );
	    }
	    parentFiber = parentFiber.sibling;
	  }
	}
	function recursivelyTraverseAtomicPassiveEffects(
	  finishedRoot$jscomp$0,
	  parentFiber
	) {
	  if (parentFiber.subtreeFlags & 10256)
	    for (parentFiber = parentFiber.child; null !== parentFiber; ) {
	      var finishedRoot = finishedRoot$jscomp$0,
	        finishedWork = parentFiber,
	        flags = finishedWork.flags;
	      switch (finishedWork.tag) {
	        case 22:
	          recursivelyTraverseAtomicPassiveEffects(finishedRoot, finishedWork);
	          flags & 2048 &&
	            commitOffscreenPassiveMountEffects(
	              finishedWork.alternate,
	              finishedWork
	            );
	          break;
	        case 24:
	          recursivelyTraverseAtomicPassiveEffects(finishedRoot, finishedWork);
	          flags & 2048 &&
	            commitCachePassiveMountEffect(finishedWork.alternate, finishedWork);
	          break;
	        default:
	          recursivelyTraverseAtomicPassiveEffects(finishedRoot, finishedWork);
	      }
	      parentFiber = parentFiber.sibling;
	    }
	}
	var suspenseyCommitFlag = 8192;
	function recursivelyAccumulateSuspenseyCommit(
	  parentFiber,
	  committedLanes,
	  suspendedState
	) {
	  if (parentFiber.subtreeFlags & suspenseyCommitFlag)
	    for (parentFiber = parentFiber.child; null !== parentFiber; )
	      accumulateSuspenseyCommitOnFiber(
	        parentFiber,
	        committedLanes,
	        suspendedState
	      ),
	        (parentFiber = parentFiber.sibling);
	}
	function accumulateSuspenseyCommitOnFiber(
	  fiber,
	  committedLanes,
	  suspendedState
	) {
	  switch (fiber.tag) {
	    case 26:
	      recursivelyAccumulateSuspenseyCommit(
	        fiber,
	        committedLanes,
	        suspendedState
	      );
	      fiber.flags & suspenseyCommitFlag &&
	        null !== fiber.memoizedState &&
	        suspendResource(
	          suspendedState,
	          currentHoistableRoot,
	          fiber.memoizedState,
	          fiber.memoizedProps
	        );
	      break;
	    case 5:
	      recursivelyAccumulateSuspenseyCommit(
	        fiber,
	        committedLanes,
	        suspendedState
	      );
	      break;
	    case 3:
	    case 4:
	      var previousHoistableRoot = currentHoistableRoot;
	      currentHoistableRoot = getHoistableRoot(fiber.stateNode.containerInfo);
	      recursivelyAccumulateSuspenseyCommit(
	        fiber,
	        committedLanes,
	        suspendedState
	      );
	      currentHoistableRoot = previousHoistableRoot;
	      break;
	    case 22:
	      null === fiber.memoizedState &&
	        ((previousHoistableRoot = fiber.alternate),
	        null !== previousHoistableRoot &&
	        null !== previousHoistableRoot.memoizedState
	          ? ((previousHoistableRoot = suspenseyCommitFlag),
	            (suspenseyCommitFlag = 16777216),
	            recursivelyAccumulateSuspenseyCommit(
	              fiber,
	              committedLanes,
	              suspendedState
	            ),
	            (suspenseyCommitFlag = previousHoistableRoot))
	          : recursivelyAccumulateSuspenseyCommit(
	              fiber,
	              committedLanes,
	              suspendedState
	            ));
	      break;
	    default:
	      recursivelyAccumulateSuspenseyCommit(
	        fiber,
	        committedLanes,
	        suspendedState
	      );
	  }
	}
	function detachAlternateSiblings(parentFiber) {
	  var previousFiber = parentFiber.alternate;
	  if (
	    null !== previousFiber &&
	    ((parentFiber = previousFiber.child), null !== parentFiber)
	  ) {
	    previousFiber.child = null;
	    do
	      (previousFiber = parentFiber.sibling),
	        (parentFiber.sibling = null),
	        (parentFiber = previousFiber);
	    while (null !== parentFiber);
	  }
	}
	function recursivelyTraversePassiveUnmountEffects(parentFiber) {
	  var deletions = parentFiber.deletions;
	  if (0 !== (parentFiber.flags & 16)) {
	    if (null !== deletions)
	      for (var i = 0; i < deletions.length; i++) {
	        var childToDelete = deletions[i];
	        nextEffect = childToDelete;
	        commitPassiveUnmountEffectsInsideOfDeletedTree_begin(
	          childToDelete,
	          parentFiber
	        );
	      }
	    detachAlternateSiblings(parentFiber);
	  }
	  if (parentFiber.subtreeFlags & 10256)
	    for (parentFiber = parentFiber.child; null !== parentFiber; )
	      commitPassiveUnmountOnFiber(parentFiber),
	        (parentFiber = parentFiber.sibling);
	}
	function commitPassiveUnmountOnFiber(finishedWork) {
	  switch (finishedWork.tag) {
	    case 0:
	    case 11:
	    case 15:
	      recursivelyTraversePassiveUnmountEffects(finishedWork);
	      finishedWork.flags & 2048 &&
	        commitHookEffectListUnmount(9, finishedWork, finishedWork.return);
	      break;
	    case 3:
	      recursivelyTraversePassiveUnmountEffects(finishedWork);
	      break;
	    case 12:
	      recursivelyTraversePassiveUnmountEffects(finishedWork);
	      break;
	    case 22:
	      var instance = finishedWork.stateNode;
	      null !== finishedWork.memoizedState &&
	      instance._visibility & 2 &&
	      (null === finishedWork.return || 13 !== finishedWork.return.tag)
	        ? ((instance._visibility &= -3),
	          recursivelyTraverseDisconnectPassiveEffects(finishedWork))
	        : recursivelyTraversePassiveUnmountEffects(finishedWork);
	      break;
	    default:
	      recursivelyTraversePassiveUnmountEffects(finishedWork);
	  }
	}
	function recursivelyTraverseDisconnectPassiveEffects(parentFiber) {
	  var deletions = parentFiber.deletions;
	  if (0 !== (parentFiber.flags & 16)) {
	    if (null !== deletions)
	      for (var i = 0; i < deletions.length; i++) {
	        var childToDelete = deletions[i];
	        nextEffect = childToDelete;
	        commitPassiveUnmountEffectsInsideOfDeletedTree_begin(
	          childToDelete,
	          parentFiber
	        );
	      }
	    detachAlternateSiblings(parentFiber);
	  }
	  for (parentFiber = parentFiber.child; null !== parentFiber; ) {
	    deletions = parentFiber;
	    switch (deletions.tag) {
	      case 0:
	      case 11:
	      case 15:
	        commitHookEffectListUnmount(8, deletions, deletions.return);
	        recursivelyTraverseDisconnectPassiveEffects(deletions);
	        break;
	      case 22:
	        i = deletions.stateNode;
	        i._visibility & 2 &&
	          ((i._visibility &= -3),
	          recursivelyTraverseDisconnectPassiveEffects(deletions));
	        break;
	      default:
	        recursivelyTraverseDisconnectPassiveEffects(deletions);
	    }
	    parentFiber = parentFiber.sibling;
	  }
	}
	function commitPassiveUnmountEffectsInsideOfDeletedTree_begin(
	  deletedSubtreeRoot,
	  nearestMountedAncestor
	) {
	  for (; null !== nextEffect; ) {
	    var fiber = nextEffect;
	    switch (fiber.tag) {
	      case 0:
	      case 11:
	      case 15:
	        commitHookEffectListUnmount(8, fiber, nearestMountedAncestor);
	        break;
	      case 23:
	      case 22:
	        if (
	          null !== fiber.memoizedState &&
	          null !== fiber.memoizedState.cachePool
	        ) {
	          var cache = fiber.memoizedState.cachePool.pool;
	          null != cache && cache.refCount++;
	        }
	        break;
	      case 24:
	        releaseCache(fiber.memoizedState.cache);
	    }
	    cache = fiber.child;
	    if (null !== cache) (cache.return = fiber), (nextEffect = cache);
	    else
	      a: for (fiber = deletedSubtreeRoot; null !== nextEffect; ) {
	        cache = nextEffect;
	        var sibling = cache.sibling,
	          returnFiber = cache.return;
	        detachFiberAfterEffects(cache);
	        if (cache === fiber) {
	          nextEffect = null;
	          break a;
	        }
	        if (null !== sibling) {
	          sibling.return = returnFiber;
	          nextEffect = sibling;
	          break a;
	        }
	        nextEffect = returnFiber;
	      }
	  }
	}
	var DefaultAsyncDispatcher = {
	    getCacheForType: function (resourceType) {
	      var cache = readContext(CacheContext),
	        cacheForType = cache.data.get(resourceType);
	      void 0 === cacheForType &&
	        ((cacheForType = resourceType()),
	        cache.data.set(resourceType, cacheForType));
	      return cacheForType;
	    },
	    cacheSignal: function () {
	      return readContext(CacheContext).controller.signal;
	    }
	  },
	  PossiblyWeakMap = "function" === typeof WeakMap ? WeakMap : Map,
	  executionContext = 0,
	  workInProgressRoot = null,
	  workInProgress = null,
	  workInProgressRootRenderLanes = 0,
	  workInProgressSuspendedReason = 0,
	  workInProgressThrownValue = null,
	  workInProgressRootDidSkipSuspendedSiblings = !1,
	  workInProgressRootIsPrerendering = !1,
	  workInProgressRootDidAttachPingListener = !1,
	  entangledRenderLanes = 0,
	  workInProgressRootExitStatus = 0,
	  workInProgressRootSkippedLanes = 0,
	  workInProgressRootInterleavedUpdatedLanes = 0,
	  workInProgressRootPingedLanes = 0,
	  workInProgressDeferredLane = 0,
	  workInProgressSuspendedRetryLanes = 0,
	  workInProgressRootConcurrentErrors = null,
	  workInProgressRootRecoverableErrors = null,
	  workInProgressRootDidIncludeRecursiveRenderUpdate = !1,
	  globalMostRecentFallbackTime = 0,
	  globalMostRecentTransitionTime = 0,
	  workInProgressRootRenderTargetTime = Infinity,
	  workInProgressTransitions = null,
	  legacyErrorBoundariesThatAlreadyFailed = null,
	  pendingEffectsStatus = 0,
	  pendingEffectsRoot = null,
	  pendingFinishedWork = null,
	  pendingEffectsLanes = 0,
	  pendingEffectsRemainingLanes = 0,
	  pendingPassiveTransitions = null,
	  pendingRecoverableErrors = null,
	  nestedUpdateCount = 0,
	  rootWithNestedUpdates = null;
	function requestUpdateLane() {
	  return 0 !== (executionContext & 2) && 0 !== workInProgressRootRenderLanes
	    ? workInProgressRootRenderLanes & -workInProgressRootRenderLanes
	    : null !== ReactSharedInternals.T
	      ? requestTransitionLane()
	      : resolveUpdatePriority();
	}
	function requestDeferredLane() {
	  if (0 === workInProgressDeferredLane)
	    if (0 === (workInProgressRootRenderLanes & 536870912) || isHydrating) {
	      var lane = nextTransitionDeferredLane;
	      nextTransitionDeferredLane <<= 1;
	      0 === (nextTransitionDeferredLane & 3932160) &&
	        (nextTransitionDeferredLane = 262144);
	      workInProgressDeferredLane = lane;
	    } else workInProgressDeferredLane = 536870912;
	  lane = suspenseHandlerStackCursor.current;
	  null !== lane && (lane.flags |= 32);
	  return workInProgressDeferredLane;
	}
	function scheduleUpdateOnFiber(root, fiber, lane) {
	  if (
	    (root === workInProgressRoot &&
	      (2 === workInProgressSuspendedReason ||
	        9 === workInProgressSuspendedReason)) ||
	    null !== root.cancelPendingCommit
	  )
	    prepareFreshStack(root, 0),
	      markRootSuspended(
	        root,
	        workInProgressRootRenderLanes,
	        workInProgressDeferredLane,
	        !1
	      );
	  markRootUpdated$1(root, lane);
	  if (0 === (executionContext & 2) || root !== workInProgressRoot)
	    root === workInProgressRoot &&
	      (0 === (executionContext & 2) &&
	        (workInProgressRootInterleavedUpdatedLanes |= lane),
	      4 === workInProgressRootExitStatus &&
	        markRootSuspended(
	          root,
	          workInProgressRootRenderLanes,
	          workInProgressDeferredLane,
	          !1
	        )),
	      ensureRootIsScheduled(root);
	}
	function performWorkOnRoot(root$jscomp$0, lanes, forceSync) {
	  if (0 !== (executionContext & 6)) throw Error(formatProdErrorMessage(327));
	  var shouldTimeSlice =
	      (!forceSync &&
	        0 === (lanes & 127) &&
	        0 === (lanes & root$jscomp$0.expiredLanes)) ||
	      checkIfRootIsPrerendering(root$jscomp$0, lanes),
	    exitStatus = shouldTimeSlice
	      ? renderRootConcurrent(root$jscomp$0, lanes)
	      : renderRootSync(root$jscomp$0, lanes, !0),
	    renderWasConcurrent = shouldTimeSlice;
	  do {
	    if (0 === exitStatus) {
	      workInProgressRootIsPrerendering &&
	        !shouldTimeSlice &&
	        markRootSuspended(root$jscomp$0, lanes, 0, !1);
	      break;
	    } else {
	      forceSync = root$jscomp$0.current.alternate;
	      if (
	        renderWasConcurrent &&
	        !isRenderConsistentWithExternalStores(forceSync)
	      ) {
	        exitStatus = renderRootSync(root$jscomp$0, lanes, !1);
	        renderWasConcurrent = !1;
	        continue;
	      }
	      if (2 === exitStatus) {
	        renderWasConcurrent = lanes;
	        if (root$jscomp$0.errorRecoveryDisabledLanes & renderWasConcurrent)
	          var JSCompiler_inline_result = 0;
	        else
	          (JSCompiler_inline_result = root$jscomp$0.pendingLanes & -536870913),
	            (JSCompiler_inline_result =
	              0 !== JSCompiler_inline_result
	                ? JSCompiler_inline_result
	                : JSCompiler_inline_result & 536870912
	                  ? 536870912
	                  : 0);
	        if (0 !== JSCompiler_inline_result) {
	          lanes = JSCompiler_inline_result;
	          a: {
	            var root = root$jscomp$0;
	            exitStatus = workInProgressRootConcurrentErrors;
	            var wasRootDehydrated = root.current.memoizedState.isDehydrated;
	            wasRootDehydrated &&
	              (prepareFreshStack(root, JSCompiler_inline_result).flags |= 256);
	            JSCompiler_inline_result = renderRootSync(
	              root,
	              JSCompiler_inline_result,
	              !1
	            );
	            if (2 !== JSCompiler_inline_result) {
	              if (
	                workInProgressRootDidAttachPingListener &&
	                !wasRootDehydrated
	              ) {
	                root.errorRecoveryDisabledLanes |= renderWasConcurrent;
	                workInProgressRootInterleavedUpdatedLanes |=
	                  renderWasConcurrent;
	                exitStatus = 4;
	                break a;
	              }
	              renderWasConcurrent = workInProgressRootRecoverableErrors;
	              workInProgressRootRecoverableErrors = exitStatus;
	              null !== renderWasConcurrent &&
	                (null === workInProgressRootRecoverableErrors
	                  ? (workInProgressRootRecoverableErrors = renderWasConcurrent)
	                  : workInProgressRootRecoverableErrors.push.apply(
	                      workInProgressRootRecoverableErrors,
	                      renderWasConcurrent
	                    ));
	            }
	            exitStatus = JSCompiler_inline_result;
	          }
	          renderWasConcurrent = !1;
	          if (2 !== exitStatus) continue;
	        }
	      }
	      if (1 === exitStatus) {
	        prepareFreshStack(root$jscomp$0, 0);
	        markRootSuspended(root$jscomp$0, lanes, 0, !0);
	        break;
	      }
	      a: {
	        shouldTimeSlice = root$jscomp$0;
	        renderWasConcurrent = exitStatus;
	        switch (renderWasConcurrent) {
	          case 0:
	          case 1:
	            throw Error(formatProdErrorMessage(345));
	          case 4:
	            if ((lanes & 4194048) !== lanes) break;
	          case 6:
	            markRootSuspended(
	              shouldTimeSlice,
	              lanes,
	              workInProgressDeferredLane,
	              !workInProgressRootDidSkipSuspendedSiblings
	            );
	            break a;
	          case 2:
	            workInProgressRootRecoverableErrors = null;
	            break;
	          case 3:
	          case 5:
	            break;
	          default:
	            throw Error(formatProdErrorMessage(329));
	        }
	        if (
	          (lanes & 62914560) === lanes &&
	          ((exitStatus = globalMostRecentFallbackTime + 300 - now()),
	          10 < exitStatus)
	        ) {
	          markRootSuspended(
	            shouldTimeSlice,
	            lanes,
	            workInProgressDeferredLane,
	            !workInProgressRootDidSkipSuspendedSiblings
	          );
	          if (0 !== getNextLanes(shouldTimeSlice, 0, !0)) break a;
	          pendingEffectsLanes = lanes;
	          shouldTimeSlice.timeoutHandle = scheduleTimeout(
	            commitRootWhenReady.bind(
	              null,
	              shouldTimeSlice,
	              forceSync,
	              workInProgressRootRecoverableErrors,
	              workInProgressTransitions,
	              workInProgressRootDidIncludeRecursiveRenderUpdate,
	              lanes,
	              workInProgressDeferredLane,
	              workInProgressRootInterleavedUpdatedLanes,
	              workInProgressSuspendedRetryLanes,
	              workInProgressRootDidSkipSuspendedSiblings,
	              renderWasConcurrent,
	              "Throttled",
	              -0,
	              0
	            ),
	            exitStatus
	          );
	          break a;
	        }
	        commitRootWhenReady(
	          shouldTimeSlice,
	          forceSync,
	          workInProgressRootRecoverableErrors,
	          workInProgressTransitions,
	          workInProgressRootDidIncludeRecursiveRenderUpdate,
	          lanes,
	          workInProgressDeferredLane,
	          workInProgressRootInterleavedUpdatedLanes,
	          workInProgressSuspendedRetryLanes,
	          workInProgressRootDidSkipSuspendedSiblings,
	          renderWasConcurrent,
	          null,
	          -0,
	          0
	        );
	      }
	    }
	    break;
	  } while (1);
	  ensureRootIsScheduled(root$jscomp$0);
	}
	function commitRootWhenReady(
	  root,
	  finishedWork,
	  recoverableErrors,
	  transitions,
	  didIncludeRenderPhaseUpdate,
	  lanes,
	  spawnedLane,
	  updatedLanes,
	  suspendedRetryLanes,
	  didSkipSuspendedSiblings,
	  exitStatus,
	  suspendedCommitReason,
	  completedRenderStartTime,
	  completedRenderEndTime
	) {
	  root.timeoutHandle = -1;
	  suspendedCommitReason = finishedWork.subtreeFlags;
	  if (
	    suspendedCommitReason & 8192 ||
	    16785408 === (suspendedCommitReason & 16785408)
	  ) {
	    suspendedCommitReason = {
	      stylesheets: null,
	      count: 0,
	      imgCount: 0,
	      imgBytes: 0,
	      suspenseyImages: [],
	      waitingForImages: !0,
	      waitingForViewTransition: !1,
	      unsuspend: noop$1
	    };
	    accumulateSuspenseyCommitOnFiber(
	      finishedWork,
	      lanes,
	      suspendedCommitReason
	    );
	    var timeoutOffset =
	      (lanes & 62914560) === lanes
	        ? globalMostRecentFallbackTime - now()
	        : (lanes & 4194048) === lanes
	          ? globalMostRecentTransitionTime - now()
	          : 0;
	    timeoutOffset = waitForCommitToBeReady(
	      suspendedCommitReason,
	      timeoutOffset
	    );
	    if (null !== timeoutOffset) {
	      pendingEffectsLanes = lanes;
	      root.cancelPendingCommit = timeoutOffset(
	        commitRoot.bind(
	          null,
	          root,
	          finishedWork,
	          lanes,
	          recoverableErrors,
	          transitions,
	          didIncludeRenderPhaseUpdate,
	          spawnedLane,
	          updatedLanes,
	          suspendedRetryLanes,
	          exitStatus,
	          suspendedCommitReason,
	          null,
	          completedRenderStartTime,
	          completedRenderEndTime
	        )
	      );
	      markRootSuspended(root, lanes, spawnedLane, !didSkipSuspendedSiblings);
	      return;
	    }
	  }
	  commitRoot(
	    root,
	    finishedWork,
	    lanes,
	    recoverableErrors,
	    transitions,
	    didIncludeRenderPhaseUpdate,
	    spawnedLane,
	    updatedLanes,
	    suspendedRetryLanes
	  );
	}
	function isRenderConsistentWithExternalStores(finishedWork) {
	  for (var node = finishedWork; ; ) {
	    var tag = node.tag;
	    if (
	      (0 === tag || 11 === tag || 15 === tag) &&
	      node.flags & 16384 &&
	      ((tag = node.updateQueue),
	      null !== tag && ((tag = tag.stores), null !== tag))
	    )
	      for (var i = 0; i < tag.length; i++) {
	        var check = tag[i],
	          getSnapshot = check.getSnapshot;
	        check = check.value;
	        try {
	          if (!objectIs(getSnapshot(), check)) return !1;
	        } catch (error) {
	          return !1;
	        }
	      }
	    tag = node.child;
	    if (node.subtreeFlags & 16384 && null !== tag)
	      (tag.return = node), (node = tag);
	    else {
	      if (node === finishedWork) break;
	      for (; null === node.sibling; ) {
	        if (null === node.return || node.return === finishedWork) return !0;
	        node = node.return;
	      }
	      node.sibling.return = node.return;
	      node = node.sibling;
	    }
	  }
	  return !0;
	}
	function markRootSuspended(
	  root,
	  suspendedLanes,
	  spawnedLane,
	  didAttemptEntireTree
	) {
	  suspendedLanes &= ~workInProgressRootPingedLanes;
	  suspendedLanes &= ~workInProgressRootInterleavedUpdatedLanes;
	  root.suspendedLanes |= suspendedLanes;
	  root.pingedLanes &= ~suspendedLanes;
	  didAttemptEntireTree && (root.warmLanes |= suspendedLanes);
	  didAttemptEntireTree = root.expirationTimes;
	  for (var lanes = suspendedLanes; 0 < lanes; ) {
	    var index$6 = 31 - clz32(lanes),
	      lane = 1 << index$6;
	    didAttemptEntireTree[index$6] = -1;
	    lanes &= ~lane;
	  }
	  0 !== spawnedLane &&
	    markSpawnedDeferredLane(root, spawnedLane, suspendedLanes);
	}
	function flushSyncWork$1() {
	  return 0 === (executionContext & 6)
	    ? (flushSyncWorkAcrossRoots_impl(0, !1), !1)
	    : !0;
	}
	function resetWorkInProgressStack() {
	  if (null !== workInProgress) {
	    if (0 === workInProgressSuspendedReason)
	      var interruptedWork = workInProgress.return;
	    else
	      (interruptedWork = workInProgress),
	        (lastContextDependency = currentlyRenderingFiber$1 = null),
	        resetHooksOnUnwind(interruptedWork),
	        (thenableState$1 = null),
	        (thenableIndexCounter$1 = 0),
	        (interruptedWork = workInProgress);
	    for (; null !== interruptedWork; )
	      unwindInterruptedWork(interruptedWork.alternate, interruptedWork),
	        (interruptedWork = interruptedWork.return);
	    workInProgress = null;
	  }
	}
	function prepareFreshStack(root, lanes) {
	  var timeoutHandle = root.timeoutHandle;
	  -1 !== timeoutHandle &&
	    ((root.timeoutHandle = -1), cancelTimeout(timeoutHandle));
	  timeoutHandle = root.cancelPendingCommit;
	  null !== timeoutHandle &&
	    ((root.cancelPendingCommit = null), timeoutHandle());
	  pendingEffectsLanes = 0;
	  resetWorkInProgressStack();
	  workInProgressRoot = root;
	  workInProgress = timeoutHandle = createWorkInProgress(root.current, null);
	  workInProgressRootRenderLanes = lanes;
	  workInProgressSuspendedReason = 0;
	  workInProgressThrownValue = null;
	  workInProgressRootDidSkipSuspendedSiblings = !1;
	  workInProgressRootIsPrerendering = checkIfRootIsPrerendering(root, lanes);
	  workInProgressRootDidAttachPingListener = !1;
	  workInProgressSuspendedRetryLanes =
	    workInProgressDeferredLane =
	    workInProgressRootPingedLanes =
	    workInProgressRootInterleavedUpdatedLanes =
	    workInProgressRootSkippedLanes =
	    workInProgressRootExitStatus =
	      0;
	  workInProgressRootRecoverableErrors = workInProgressRootConcurrentErrors =
	    null;
	  workInProgressRootDidIncludeRecursiveRenderUpdate = !1;
	  0 !== (lanes & 8) && (lanes |= lanes & 32);
	  var allEntangledLanes = root.entangledLanes;
	  if (0 !== allEntangledLanes)
	    for (
	      root = root.entanglements, allEntangledLanes &= lanes;
	      0 < allEntangledLanes;

	    ) {
	      var index$4 = 31 - clz32(allEntangledLanes),
	        lane = 1 << index$4;
	      lanes |= root[index$4];
	      allEntangledLanes &= ~lane;
	    }
	  entangledRenderLanes = lanes;
	  finishQueueingConcurrentUpdates();
	  return timeoutHandle;
	}
	function handleThrow(root, thrownValue) {
	  currentlyRenderingFiber = null;
	  ReactSharedInternals.H = ContextOnlyDispatcher;
	  thrownValue === SuspenseException || thrownValue === SuspenseActionException
	    ? ((thrownValue = getSuspendedThenable()),
	      (workInProgressSuspendedReason = 3))
	    : thrownValue === SuspenseyCommitException
	      ? ((thrownValue = getSuspendedThenable()),
	        (workInProgressSuspendedReason = 4))
	      : (workInProgressSuspendedReason =
	          thrownValue === SelectiveHydrationException
	            ? 8
	            : null !== thrownValue &&
	                "object" === typeof thrownValue &&
	                "function" === typeof thrownValue.then
	              ? 6
	              : 1);
	  workInProgressThrownValue = thrownValue;
	  null === workInProgress &&
	    ((workInProgressRootExitStatus = 1),
	    logUncaughtError(
	      root,
	      createCapturedValueAtFiber(thrownValue, root.current)
	    ));
	}
	function shouldRemainOnPreviousScreen() {
	  var handler = suspenseHandlerStackCursor.current;
	  return null === handler
	    ? !0
	    : (workInProgressRootRenderLanes & 4194048) ===
	        workInProgressRootRenderLanes
	      ? null === shellBoundary
	        ? !0
	        : !1
	      : (workInProgressRootRenderLanes & 62914560) ===
	            workInProgressRootRenderLanes ||
	          0 !== (workInProgressRootRenderLanes & 536870912)
	        ? handler === shellBoundary
	        : !1;
	}
	function pushDispatcher() {
	  var prevDispatcher = ReactSharedInternals.H;
	  ReactSharedInternals.H = ContextOnlyDispatcher;
	  return null === prevDispatcher ? ContextOnlyDispatcher : prevDispatcher;
	}
	function pushAsyncDispatcher() {
	  var prevAsyncDispatcher = ReactSharedInternals.A;
	  ReactSharedInternals.A = DefaultAsyncDispatcher;
	  return prevAsyncDispatcher;
	}
	function renderDidSuspendDelayIfPossible() {
	  workInProgressRootExitStatus = 4;
	  workInProgressRootDidSkipSuspendedSiblings ||
	    ((workInProgressRootRenderLanes & 4194048) !==
	      workInProgressRootRenderLanes &&
	      null !== suspenseHandlerStackCursor.current) ||
	    (workInProgressRootIsPrerendering = !0);
	  (0 === (workInProgressRootSkippedLanes & 134217727) &&
	    0 === (workInProgressRootInterleavedUpdatedLanes & 134217727)) ||
	    null === workInProgressRoot ||
	    markRootSuspended(
	      workInProgressRoot,
	      workInProgressRootRenderLanes,
	      workInProgressDeferredLane,
	      !1
	    );
	}
	function renderRootSync(root, lanes, shouldYieldForPrerendering) {
	  var prevExecutionContext = executionContext;
	  executionContext |= 2;
	  var prevDispatcher = pushDispatcher(),
	    prevAsyncDispatcher = pushAsyncDispatcher();
	  if (workInProgressRoot !== root || workInProgressRootRenderLanes !== lanes)
	    (workInProgressTransitions = null), prepareFreshStack(root, lanes);
	  lanes = !1;
	  var exitStatus = workInProgressRootExitStatus;
	  a: do
	    try {
	      if (0 !== workInProgressSuspendedReason && null !== workInProgress) {
	        var unitOfWork = workInProgress,
	          thrownValue = workInProgressThrownValue;
	        switch (workInProgressSuspendedReason) {
	          case 8:
	            resetWorkInProgressStack();
	            exitStatus = 6;
	            break a;
	          case 3:
	          case 2:
	          case 9:
	          case 6:
	            null === suspenseHandlerStackCursor.current && (lanes = !0);
	            var reason = workInProgressSuspendedReason;
	            workInProgressSuspendedReason = 0;
	            workInProgressThrownValue = null;
	            throwAndUnwindWorkLoop(root, unitOfWork, thrownValue, reason);
	            if (
	              shouldYieldForPrerendering &&
	              workInProgressRootIsPrerendering
	            ) {
	              exitStatus = 0;
	              break a;
	            }
	            break;
	          default:
	            (reason = workInProgressSuspendedReason),
	              (workInProgressSuspendedReason = 0),
	              (workInProgressThrownValue = null),
	              throwAndUnwindWorkLoop(root, unitOfWork, thrownValue, reason);
	        }
	      }
	      workLoopSync();
	      exitStatus = workInProgressRootExitStatus;
	      break;
	    } catch (thrownValue$165) {
	      handleThrow(root, thrownValue$165);
	    }
	  while (1);
	  lanes && root.shellSuspendCounter++;
	  lastContextDependency = currentlyRenderingFiber$1 = null;
	  executionContext = prevExecutionContext;
	  ReactSharedInternals.H = prevDispatcher;
	  ReactSharedInternals.A = prevAsyncDispatcher;
	  null === workInProgress &&
	    ((workInProgressRoot = null),
	    (workInProgressRootRenderLanes = 0),
	    finishQueueingConcurrentUpdates());
	  return exitStatus;
	}
	function workLoopSync() {
	  for (; null !== workInProgress; ) performUnitOfWork(workInProgress);
	}
	function renderRootConcurrent(root, lanes) {
	  var prevExecutionContext = executionContext;
	  executionContext |= 2;
	  var prevDispatcher = pushDispatcher(),
	    prevAsyncDispatcher = pushAsyncDispatcher();
	  workInProgressRoot !== root || workInProgressRootRenderLanes !== lanes
	    ? ((workInProgressTransitions = null),
	      (workInProgressRootRenderTargetTime = now() + 500),
	      prepareFreshStack(root, lanes))
	    : (workInProgressRootIsPrerendering = checkIfRootIsPrerendering(
	        root,
	        lanes
	      ));
	  a: do
	    try {
	      if (0 !== workInProgressSuspendedReason && null !== workInProgress) {
	        lanes = workInProgress;
	        var thrownValue = workInProgressThrownValue;
	        b: switch (workInProgressSuspendedReason) {
	          case 1:
	            workInProgressSuspendedReason = 0;
	            workInProgressThrownValue = null;
	            throwAndUnwindWorkLoop(root, lanes, thrownValue, 1);
	            break;
	          case 2:
	          case 9:
	            if (isThenableResolved(thrownValue)) {
	              workInProgressSuspendedReason = 0;
	              workInProgressThrownValue = null;
	              replaySuspendedUnitOfWork(lanes);
	              break;
	            }
	            lanes = function () {
	              (2 !== workInProgressSuspendedReason &&
	                9 !== workInProgressSuspendedReason) ||
	                workInProgressRoot !== root ||
	                (workInProgressSuspendedReason = 7);
	              ensureRootIsScheduled(root);
	            };
	            thrownValue.then(lanes, lanes);
	            break a;
	          case 3:
	            workInProgressSuspendedReason = 7;
	            break a;
	          case 4:
	            workInProgressSuspendedReason = 5;
	            break a;
	          case 7:
	            isThenableResolved(thrownValue)
	              ? ((workInProgressSuspendedReason = 0),
	                (workInProgressThrownValue = null),
	                replaySuspendedUnitOfWork(lanes))
	              : ((workInProgressSuspendedReason = 0),
	                (workInProgressThrownValue = null),
	                throwAndUnwindWorkLoop(root, lanes, thrownValue, 7));
	            break;
	          case 5:
	            var resource = null;
	            switch (workInProgress.tag) {
	              case 26:
	                resource = workInProgress.memoizedState;
	              case 5:
	              case 27:
	                var hostFiber = workInProgress;
	                if (
	                  resource
	                    ? preloadResource(resource)
	                    : hostFiber.stateNode.complete
	                ) {
	                  workInProgressSuspendedReason = 0;
	                  workInProgressThrownValue = null;
	                  var sibling = hostFiber.sibling;
	                  if (null !== sibling) workInProgress = sibling;
	                  else {
	                    var returnFiber = hostFiber.return;
	                    null !== returnFiber
	                      ? ((workInProgress = returnFiber),
	                        completeUnitOfWork(returnFiber))
	                      : (workInProgress = null);
	                  }
	                  break b;
	                }
	            }
	            workInProgressSuspendedReason = 0;
	            workInProgressThrownValue = null;
	            throwAndUnwindWorkLoop(root, lanes, thrownValue, 5);
	            break;
	          case 6:
	            workInProgressSuspendedReason = 0;
	            workInProgressThrownValue = null;
	            throwAndUnwindWorkLoop(root, lanes, thrownValue, 6);
	            break;
	          case 8:
	            resetWorkInProgressStack();
	            workInProgressRootExitStatus = 6;
	            break a;
	          default:
	            throw Error(formatProdErrorMessage(462));
	        }
	      }
	      workLoopConcurrentByScheduler();
	      break;
	    } catch (thrownValue$167) {
	      handleThrow(root, thrownValue$167);
	    }
	  while (1);
	  lastContextDependency = currentlyRenderingFiber$1 = null;
	  ReactSharedInternals.H = prevDispatcher;
	  ReactSharedInternals.A = prevAsyncDispatcher;
	  executionContext = prevExecutionContext;
	  if (null !== workInProgress) return 0;
	  workInProgressRoot = null;
	  workInProgressRootRenderLanes = 0;
	  finishQueueingConcurrentUpdates();
	  return workInProgressRootExitStatus;
	}
	function workLoopConcurrentByScheduler() {
	  for (; null !== workInProgress && !shouldYield(); )
	    performUnitOfWork(workInProgress);
	}
	function performUnitOfWork(unitOfWork) {
	  var next = beginWork(unitOfWork.alternate, unitOfWork, entangledRenderLanes);
	  unitOfWork.memoizedProps = unitOfWork.pendingProps;
	  null === next ? completeUnitOfWork(unitOfWork) : (workInProgress = next);
	}
	function replaySuspendedUnitOfWork(unitOfWork) {
	  var next = unitOfWork;
	  var current = next.alternate;
	  switch (next.tag) {
	    case 15:
	    case 0:
	      next = replayFunctionComponent(
	        current,
	        next,
	        next.pendingProps,
	        next.type,
	        void 0,
	        workInProgressRootRenderLanes
	      );
	      break;
	    case 11:
	      next = replayFunctionComponent(
	        current,
	        next,
	        next.pendingProps,
	        next.type.render,
	        next.ref,
	        workInProgressRootRenderLanes
	      );
	      break;
	    case 5:
	      resetHooksOnUnwind(next);
	    default:
	      unwindInterruptedWork(current, next),
	        (next = workInProgress =
	          resetWorkInProgress(next, entangledRenderLanes)),
	        (next = beginWork(current, next, entangledRenderLanes));
	  }
	  unitOfWork.memoizedProps = unitOfWork.pendingProps;
	  null === next ? completeUnitOfWork(unitOfWork) : (workInProgress = next);
	}
	function throwAndUnwindWorkLoop(
	  root,
	  unitOfWork,
	  thrownValue,
	  suspendedReason
	) {
	  lastContextDependency = currentlyRenderingFiber$1 = null;
	  resetHooksOnUnwind(unitOfWork);
	  thenableState$1 = null;
	  thenableIndexCounter$1 = 0;
	  var returnFiber = unitOfWork.return;
	  try {
	    if (
	      throwException(
	        root,
	        returnFiber,
	        unitOfWork,
	        thrownValue,
	        workInProgressRootRenderLanes
	      )
	    ) {
	      workInProgressRootExitStatus = 1;
	      logUncaughtError(
	        root,
	        createCapturedValueAtFiber(thrownValue, root.current)
	      );
	      workInProgress = null;
	      return;
	    }
	  } catch (error) {
	    if (null !== returnFiber) throw ((workInProgress = returnFiber), error);
	    workInProgressRootExitStatus = 1;
	    logUncaughtError(
	      root,
	      createCapturedValueAtFiber(thrownValue, root.current)
	    );
	    workInProgress = null;
	    return;
	  }
	  if (unitOfWork.flags & 32768) {
	    if (isHydrating || 1 === suspendedReason) root = !0;
	    else if (
	      workInProgressRootIsPrerendering ||
	      0 !== (workInProgressRootRenderLanes & 536870912)
	    )
	      root = !1;
	    else if (
	      ((workInProgressRootDidSkipSuspendedSiblings = root = !0),
	      2 === suspendedReason ||
	        9 === suspendedReason ||
	        3 === suspendedReason ||
	        6 === suspendedReason)
	    )
	      (suspendedReason = suspenseHandlerStackCursor.current),
	        null !== suspendedReason &&
	          13 === suspendedReason.tag &&
	          (suspendedReason.flags |= 16384);
	    unwindUnitOfWork(unitOfWork, root);
	  } else completeUnitOfWork(unitOfWork);
	}
	function completeUnitOfWork(unitOfWork) {
	  var completedWork = unitOfWork;
	  do {
	    if (0 !== (completedWork.flags & 32768)) {
	      unwindUnitOfWork(
	        completedWork,
	        workInProgressRootDidSkipSuspendedSiblings
	      );
	      return;
	    }
	    unitOfWork = completedWork.return;
	    var next = completeWork(
	      completedWork.alternate,
	      completedWork,
	      entangledRenderLanes
	    );
	    if (null !== next) {
	      workInProgress = next;
	      return;
	    }
	    completedWork = completedWork.sibling;
	    if (null !== completedWork) {
	      workInProgress = completedWork;
	      return;
	    }
	    workInProgress = completedWork = unitOfWork;
	  } while (null !== completedWork);
	  0 === workInProgressRootExitStatus && (workInProgressRootExitStatus = 5);
	}
	function unwindUnitOfWork(unitOfWork, skipSiblings) {
	  do {
	    var next = unwindWork(unitOfWork.alternate, unitOfWork);
	    if (null !== next) {
	      next.flags &= 32767;
	      workInProgress = next;
	      return;
	    }
	    next = unitOfWork.return;
	    null !== next &&
	      ((next.flags |= 32768), (next.subtreeFlags = 0), (next.deletions = null));
	    if (
	      !skipSiblings &&
	      ((unitOfWork = unitOfWork.sibling), null !== unitOfWork)
	    ) {
	      workInProgress = unitOfWork;
	      return;
	    }
	    workInProgress = unitOfWork = next;
	  } while (null !== unitOfWork);
	  workInProgressRootExitStatus = 6;
	  workInProgress = null;
	}
	function commitRoot(
	  root,
	  finishedWork,
	  lanes,
	  recoverableErrors,
	  transitions,
	  didIncludeRenderPhaseUpdate,
	  spawnedLane,
	  updatedLanes,
	  suspendedRetryLanes
	) {
	  root.cancelPendingCommit = null;
	  do flushPendingEffects();
	  while (0 !== pendingEffectsStatus);
	  if (0 !== (executionContext & 6)) throw Error(formatProdErrorMessage(327));
	  if (null !== finishedWork) {
	    if (finishedWork === root.current) throw Error(formatProdErrorMessage(177));
	    didIncludeRenderPhaseUpdate = finishedWork.lanes | finishedWork.childLanes;
	    didIncludeRenderPhaseUpdate |= concurrentlyUpdatedLanes;
	    markRootFinished(
	      root,
	      lanes,
	      didIncludeRenderPhaseUpdate,
	      spawnedLane,
	      updatedLanes,
	      suspendedRetryLanes
	    );
	    root === workInProgressRoot &&
	      ((workInProgress = workInProgressRoot = null),
	      (workInProgressRootRenderLanes = 0));
	    pendingFinishedWork = finishedWork;
	    pendingEffectsRoot = root;
	    pendingEffectsLanes = lanes;
	    pendingEffectsRemainingLanes = didIncludeRenderPhaseUpdate;
	    pendingPassiveTransitions = transitions;
	    pendingRecoverableErrors = recoverableErrors;
	    0 !== (finishedWork.subtreeFlags & 10256) ||
	    0 !== (finishedWork.flags & 10256)
	      ? ((root.callbackNode = null),
	        (root.callbackPriority = 0),
	        scheduleCallback$1(NormalPriority$1, function () {
	          flushPassiveEffects();
	          return null;
	        }))
	      : ((root.callbackNode = null), (root.callbackPriority = 0));
	    recoverableErrors = 0 !== (finishedWork.flags & 13878);
	    if (0 !== (finishedWork.subtreeFlags & 13878) || recoverableErrors) {
	      recoverableErrors = ReactSharedInternals.T;
	      ReactSharedInternals.T = null;
	      transitions = ReactDOMSharedInternals.p;
	      ReactDOMSharedInternals.p = 2;
	      spawnedLane = executionContext;
	      executionContext |= 4;
	      try {
	        commitBeforeMutationEffects(root, finishedWork, lanes);
	      } finally {
	        (executionContext = spawnedLane),
	          (ReactDOMSharedInternals.p = transitions),
	          (ReactSharedInternals.T = recoverableErrors);
	      }
	    }
	    pendingEffectsStatus = 1;
	    flushMutationEffects();
	    flushLayoutEffects();
	    flushSpawnedWork();
	  }
	}
	function flushMutationEffects() {
	  if (1 === pendingEffectsStatus) {
	    pendingEffectsStatus = 0;
	    var root = pendingEffectsRoot,
	      finishedWork = pendingFinishedWork,
	      rootMutationHasEffect = 0 !== (finishedWork.flags & 13878);
	    if (0 !== (finishedWork.subtreeFlags & 13878) || rootMutationHasEffect) {
	      rootMutationHasEffect = ReactSharedInternals.T;
	      ReactSharedInternals.T = null;
	      var previousPriority = ReactDOMSharedInternals.p;
	      ReactDOMSharedInternals.p = 2;
	      var prevExecutionContext = executionContext;
	      executionContext |= 4;
	      try {
	        commitMutationEffectsOnFiber(finishedWork, root);
	        var priorSelectionInformation = selectionInformation,
	          curFocusedElem = getActiveElementDeep(root.containerInfo),
	          priorFocusedElem = priorSelectionInformation.focusedElem,
	          priorSelectionRange = priorSelectionInformation.selectionRange;
	        if (
	          curFocusedElem !== priorFocusedElem &&
	          priorFocusedElem &&
	          priorFocusedElem.ownerDocument &&
	          containsNode(
	            priorFocusedElem.ownerDocument.documentElement,
	            priorFocusedElem
	          )
	        ) {
	          if (
	            null !== priorSelectionRange &&
	            hasSelectionCapabilities(priorFocusedElem)
	          ) {
	            var start = priorSelectionRange.start,
	              end = priorSelectionRange.end;
	            void 0 === end && (end = start);
	            if ("selectionStart" in priorFocusedElem)
	              (priorFocusedElem.selectionStart = start),
	                (priorFocusedElem.selectionEnd = Math.min(
	                  end,
	                  priorFocusedElem.value.length
	                ));
	            else {
	              var doc = priorFocusedElem.ownerDocument || document,
	                win = (doc && doc.defaultView) || window;
	              if (win.getSelection) {
	                var selection = win.getSelection(),
	                  length = priorFocusedElem.textContent.length,
	                  start$jscomp$0 = Math.min(priorSelectionRange.start, length),
	                  end$jscomp$0 =
	                    void 0 === priorSelectionRange.end
	                      ? start$jscomp$0
	                      : Math.min(priorSelectionRange.end, length);
	                !selection.extend &&
	                  start$jscomp$0 > end$jscomp$0 &&
	                  ((curFocusedElem = end$jscomp$0),
	                  (end$jscomp$0 = start$jscomp$0),
	                  (start$jscomp$0 = curFocusedElem));
	                var startMarker = getNodeForCharacterOffset(
	                    priorFocusedElem,
	                    start$jscomp$0
	                  ),
	                  endMarker = getNodeForCharacterOffset(
	                    priorFocusedElem,
	                    end$jscomp$0
	                  );
	                if (
	                  startMarker &&
	                  endMarker &&
	                  (1 !== selection.rangeCount ||
	                    selection.anchorNode !== startMarker.node ||
	                    selection.anchorOffset !== startMarker.offset ||
	                    selection.focusNode !== endMarker.node ||
	                    selection.focusOffset !== endMarker.offset)
	                ) {
	                  var range = doc.createRange();
	                  range.setStart(startMarker.node, startMarker.offset);
	                  selection.removeAllRanges();
	                  start$jscomp$0 > end$jscomp$0
	                    ? (selection.addRange(range),
	                      selection.extend(endMarker.node, endMarker.offset))
	                    : (range.setEnd(endMarker.node, endMarker.offset),
	                      selection.addRange(range));
	                }
	              }
	            }
	          }
	          doc = [];
	          for (
	            selection = priorFocusedElem;
	            (selection = selection.parentNode);

	          )
	            1 === selection.nodeType &&
	              doc.push({
	                element: selection,
	                left: selection.scrollLeft,
	                top: selection.scrollTop
	              });
	          "function" === typeof priorFocusedElem.focus &&
	            priorFocusedElem.focus();
	          for (
	            priorFocusedElem = 0;
	            priorFocusedElem < doc.length;
	            priorFocusedElem++
	          ) {
	            var info = doc[priorFocusedElem];
	            info.element.scrollLeft = info.left;
	            info.element.scrollTop = info.top;
	          }
	        }
	        _enabled = !!eventsEnabled;
	        selectionInformation = eventsEnabled = null;
	      } finally {
	        (executionContext = prevExecutionContext),
	          (ReactDOMSharedInternals.p = previousPriority),
	          (ReactSharedInternals.T = rootMutationHasEffect);
	      }
	    }
	    root.current = finishedWork;
	    pendingEffectsStatus = 2;
	  }
	}
	function flushLayoutEffects() {
	  if (2 === pendingEffectsStatus) {
	    pendingEffectsStatus = 0;
	    var root = pendingEffectsRoot,
	      finishedWork = pendingFinishedWork,
	      rootHasLayoutEffect = 0 !== (finishedWork.flags & 8772);
	    if (0 !== (finishedWork.subtreeFlags & 8772) || rootHasLayoutEffect) {
	      rootHasLayoutEffect = ReactSharedInternals.T;
	      ReactSharedInternals.T = null;
	      var previousPriority = ReactDOMSharedInternals.p;
	      ReactDOMSharedInternals.p = 2;
	      var prevExecutionContext = executionContext;
	      executionContext |= 4;
	      try {
	        commitLayoutEffectOnFiber(root, finishedWork.alternate, finishedWork);
	      } finally {
	        (executionContext = prevExecutionContext),
	          (ReactDOMSharedInternals.p = previousPriority),
	          (ReactSharedInternals.T = rootHasLayoutEffect);
	      }
	    }
	    pendingEffectsStatus = 3;
	  }
	}
	function flushSpawnedWork() {
	  if (4 === pendingEffectsStatus || 3 === pendingEffectsStatus) {
	    pendingEffectsStatus = 0;
	    requestPaint();
	    var root = pendingEffectsRoot,
	      finishedWork = pendingFinishedWork,
	      lanes = pendingEffectsLanes,
	      recoverableErrors = pendingRecoverableErrors;
	    0 !== (finishedWork.subtreeFlags & 10256) ||
	    0 !== (finishedWork.flags & 10256)
	      ? (pendingEffectsStatus = 5)
	      : ((pendingEffectsStatus = 0),
	        (pendingFinishedWork = pendingEffectsRoot = null),
	        releaseRootPooledCache(root, root.pendingLanes));
	    var remainingLanes = root.pendingLanes;
	    0 === remainingLanes && (legacyErrorBoundariesThatAlreadyFailed = null);
	    lanesToEventPriority(lanes);
	    finishedWork = finishedWork.stateNode;
	    if (injectedHook && "function" === typeof injectedHook.onCommitFiberRoot)
	      try {
	        injectedHook.onCommitFiberRoot(
	          rendererID,
	          finishedWork,
	          void 0,
	          128 === (finishedWork.current.flags & 128)
	        );
	      } catch (err) {}
	    if (null !== recoverableErrors) {
	      finishedWork = ReactSharedInternals.T;
	      remainingLanes = ReactDOMSharedInternals.p;
	      ReactDOMSharedInternals.p = 2;
	      ReactSharedInternals.T = null;
	      try {
	        for (
	          var onRecoverableError = root.onRecoverableError, i = 0;
	          i < recoverableErrors.length;
	          i++
	        ) {
	          var recoverableError = recoverableErrors[i];
	          onRecoverableError(recoverableError.value, {
	            componentStack: recoverableError.stack
	          });
	        }
	      } finally {
	        (ReactSharedInternals.T = finishedWork),
	          (ReactDOMSharedInternals.p = remainingLanes);
	      }
	    }
	    0 !== (pendingEffectsLanes & 3) && flushPendingEffects();
	    ensureRootIsScheduled(root);
	    remainingLanes = root.pendingLanes;
	    0 !== (lanes & 261930) && 0 !== (remainingLanes & 42)
	      ? root === rootWithNestedUpdates
	        ? nestedUpdateCount++
	        : ((nestedUpdateCount = 0), (rootWithNestedUpdates = root))
	      : (nestedUpdateCount = 0);
	    flushSyncWorkAcrossRoots_impl(0, !1);
	  }
	}
	function releaseRootPooledCache(root, remainingLanes) {
	  0 === (root.pooledCacheLanes &= remainingLanes) &&
	    ((remainingLanes = root.pooledCache),
	    null != remainingLanes &&
	      ((root.pooledCache = null), releaseCache(remainingLanes)));
	}
	function flushPendingEffects() {
	  flushMutationEffects();
	  flushLayoutEffects();
	  flushSpawnedWork();
	  return flushPassiveEffects();
	}
	function flushPassiveEffects() {
	  if (5 !== pendingEffectsStatus) return !1;
	  var root = pendingEffectsRoot,
	    remainingLanes = pendingEffectsRemainingLanes;
	  pendingEffectsRemainingLanes = 0;
	  var renderPriority = lanesToEventPriority(pendingEffectsLanes),
	    prevTransition = ReactSharedInternals.T,
	    previousPriority = ReactDOMSharedInternals.p;
	  try {
	    ReactDOMSharedInternals.p = 32 > renderPriority ? 32 : renderPriority;
	    ReactSharedInternals.T = null;
	    renderPriority = pendingPassiveTransitions;
	    pendingPassiveTransitions = null;
	    var root$jscomp$0 = pendingEffectsRoot,
	      lanes = pendingEffectsLanes;
	    pendingEffectsStatus = 0;
	    pendingFinishedWork = pendingEffectsRoot = null;
	    pendingEffectsLanes = 0;
	    if (0 !== (executionContext & 6)) throw Error(formatProdErrorMessage(331));
	    var prevExecutionContext = executionContext;
	    executionContext |= 4;
	    commitPassiveUnmountOnFiber(root$jscomp$0.current);
	    commitPassiveMountOnFiber(
	      root$jscomp$0,
	      root$jscomp$0.current,
	      lanes,
	      renderPriority
	    );
	    executionContext = prevExecutionContext;
	    flushSyncWorkAcrossRoots_impl(0, !1);
	    if (
	      injectedHook &&
	      "function" === typeof injectedHook.onPostCommitFiberRoot
	    )
	      try {
	        injectedHook.onPostCommitFiberRoot(rendererID, root$jscomp$0);
	      } catch (err) {}
	    return !0;
	  } finally {
	    (ReactDOMSharedInternals.p = previousPriority),
	      (ReactSharedInternals.T = prevTransition),
	      releaseRootPooledCache(root, remainingLanes);
	  }
	}
	function captureCommitPhaseErrorOnRoot(rootFiber, sourceFiber, error) {
	  sourceFiber = createCapturedValueAtFiber(error, sourceFiber);
	  sourceFiber = createRootErrorUpdate(rootFiber.stateNode, sourceFiber, 2);
	  rootFiber = enqueueUpdate(rootFiber, sourceFiber, 2);
	  null !== rootFiber &&
	    (markRootUpdated$1(rootFiber, 2), ensureRootIsScheduled(rootFiber));
	}
	function captureCommitPhaseError(sourceFiber, nearestMountedAncestor, error) {
	  if (3 === sourceFiber.tag)
	    captureCommitPhaseErrorOnRoot(sourceFiber, sourceFiber, error);
	  else
	    for (; null !== nearestMountedAncestor; ) {
	      if (3 === nearestMountedAncestor.tag) {
	        captureCommitPhaseErrorOnRoot(
	          nearestMountedAncestor,
	          sourceFiber,
	          error
	        );
	        break;
	      } else if (1 === nearestMountedAncestor.tag) {
	        var instance = nearestMountedAncestor.stateNode;
	        if (
	          "function" ===
	            typeof nearestMountedAncestor.type.getDerivedStateFromError ||
	          ("function" === typeof instance.componentDidCatch &&
	            (null === legacyErrorBoundariesThatAlreadyFailed ||
	              !legacyErrorBoundariesThatAlreadyFailed.has(instance)))
	        ) {
	          sourceFiber = createCapturedValueAtFiber(error, sourceFiber);
	          error = createClassErrorUpdate(2);
	          instance = enqueueUpdate(nearestMountedAncestor, error, 2);
	          null !== instance &&
	            (initializeClassErrorUpdate(
	              error,
	              instance,
	              nearestMountedAncestor,
	              sourceFiber
	            ),
	            markRootUpdated$1(instance, 2),
	            ensureRootIsScheduled(instance));
	          break;
	        }
	      }
	      nearestMountedAncestor = nearestMountedAncestor.return;
	    }
	}
	function attachPingListener(root, wakeable, lanes) {
	  var pingCache = root.pingCache;
	  if (null === pingCache) {
	    pingCache = root.pingCache = new PossiblyWeakMap();
	    var threadIDs = new Set();
	    pingCache.set(wakeable, threadIDs);
	  } else
	    (threadIDs = pingCache.get(wakeable)),
	      void 0 === threadIDs &&
	        ((threadIDs = new Set()), pingCache.set(wakeable, threadIDs));
	  threadIDs.has(lanes) ||
	    ((workInProgressRootDidAttachPingListener = !0),
	    threadIDs.add(lanes),
	    (root = pingSuspendedRoot.bind(null, root, wakeable, lanes)),
	    wakeable.then(root, root));
	}
	function pingSuspendedRoot(root, wakeable, pingedLanes) {
	  var pingCache = root.pingCache;
	  null !== pingCache && pingCache.delete(wakeable);
	  root.pingedLanes |= root.suspendedLanes & pingedLanes;
	  root.warmLanes &= ~pingedLanes;
	  workInProgressRoot === root &&
	    (workInProgressRootRenderLanes & pingedLanes) === pingedLanes &&
	    (4 === workInProgressRootExitStatus ||
	    (3 === workInProgressRootExitStatus &&
	      (workInProgressRootRenderLanes & 62914560) ===
	        workInProgressRootRenderLanes &&
	      300 > now() - globalMostRecentFallbackTime)
	      ? 0 === (executionContext & 2) && prepareFreshStack(root, 0)
	      : (workInProgressRootPingedLanes |= pingedLanes),
	    workInProgressSuspendedRetryLanes === workInProgressRootRenderLanes &&
	      (workInProgressSuspendedRetryLanes = 0));
	  ensureRootIsScheduled(root);
	}
	function retryTimedOutBoundary(boundaryFiber, retryLane) {
	  0 === retryLane && (retryLane = claimNextRetryLane());
	  boundaryFiber = enqueueConcurrentRenderForLane(boundaryFiber, retryLane);
	  null !== boundaryFiber &&
	    (markRootUpdated$1(boundaryFiber, retryLane),
	    ensureRootIsScheduled(boundaryFiber));
	}
	function retryDehydratedSuspenseBoundary(boundaryFiber) {
	  var suspenseState = boundaryFiber.memoizedState,
	    retryLane = 0;
	  null !== suspenseState && (retryLane = suspenseState.retryLane);
	  retryTimedOutBoundary(boundaryFiber, retryLane);
	}
	function resolveRetryWakeable(boundaryFiber, wakeable) {
	  var retryLane = 0;
	  switch (boundaryFiber.tag) {
	    case 31:
	    case 13:
	      var retryCache = boundaryFiber.stateNode;
	      var suspenseState = boundaryFiber.memoizedState;
	      null !== suspenseState && (retryLane = suspenseState.retryLane);
	      break;
	    case 19:
	      retryCache = boundaryFiber.stateNode;
	      break;
	    case 22:
	      retryCache = boundaryFiber.stateNode._retryCache;
	      break;
	    default:
	      throw Error(formatProdErrorMessage(314));
	  }
	  null !== retryCache && retryCache.delete(wakeable);
	  retryTimedOutBoundary(boundaryFiber, retryLane);
	}
	function scheduleCallback$1(priorityLevel, callback) {
	  return scheduleCallback$3(priorityLevel, callback);
	}
	var firstScheduledRoot = null,
	  lastScheduledRoot = null,
	  didScheduleMicrotask = !1,
	  mightHavePendingSyncWork = !1,
	  isFlushingWork = !1,
	  currentEventTransitionLane = 0;
	function ensureRootIsScheduled(root) {
	  root !== lastScheduledRoot &&
	    null === root.next &&
	    (null === lastScheduledRoot
	      ? (firstScheduledRoot = lastScheduledRoot = root)
	      : (lastScheduledRoot = lastScheduledRoot.next = root));
	  mightHavePendingSyncWork = !0;
	  didScheduleMicrotask ||
	    ((didScheduleMicrotask = !0), scheduleImmediateRootScheduleTask());
	}
	function flushSyncWorkAcrossRoots_impl(syncTransitionLanes, onlyLegacy) {
	  if (!isFlushingWork && mightHavePendingSyncWork) {
	    isFlushingWork = !0;
	    do {
	      var didPerformSomeWork = !1;
	      for (var root$170 = firstScheduledRoot; null !== root$170; ) {
	        if (!onlyLegacy)
	          if (0 !== syncTransitionLanes) {
	            var pendingLanes = root$170.pendingLanes;
	            if (0 === pendingLanes) var JSCompiler_inline_result = 0;
	            else {
	              var suspendedLanes = root$170.suspendedLanes,
	                pingedLanes = root$170.pingedLanes;
	              JSCompiler_inline_result =
	                (1 << (31 - clz32(42 | syncTransitionLanes) + 1)) - 1;
	              JSCompiler_inline_result &=
	                pendingLanes & ~(suspendedLanes & ~pingedLanes);
	              JSCompiler_inline_result =
	                JSCompiler_inline_result & 201326741
	                  ? (JSCompiler_inline_result & 201326741) | 1
	                  : JSCompiler_inline_result
	                    ? JSCompiler_inline_result | 2
	                    : 0;
	            }
	            0 !== JSCompiler_inline_result &&
	              ((didPerformSomeWork = !0),
	              performSyncWorkOnRoot(root$170, JSCompiler_inline_result));
	          } else
	            (JSCompiler_inline_result = workInProgressRootRenderLanes),
	              (JSCompiler_inline_result = getNextLanes(
	                root$170,
	                root$170 === workInProgressRoot ? JSCompiler_inline_result : 0,
	                null !== root$170.cancelPendingCommit ||
	                  -1 !== root$170.timeoutHandle
	              )),
	              0 === (JSCompiler_inline_result & 3) ||
	                checkIfRootIsPrerendering(root$170, JSCompiler_inline_result) ||
	                ((didPerformSomeWork = !0),
	                performSyncWorkOnRoot(root$170, JSCompiler_inline_result));
	        root$170 = root$170.next;
	      }
	    } while (didPerformSomeWork);
	    isFlushingWork = !1;
	  }
	}
	function processRootScheduleInImmediateTask() {
	  processRootScheduleInMicrotask();
	}
	function processRootScheduleInMicrotask() {
	  mightHavePendingSyncWork = didScheduleMicrotask = !1;
	  var syncTransitionLanes = 0;
	  0 !== currentEventTransitionLane &&
	    shouldAttemptEagerTransition() &&
	    (syncTransitionLanes = currentEventTransitionLane);
	  for (
	    var currentTime = now(), prev = null, root = firstScheduledRoot;
	    null !== root;

	  ) {
	    var next = root.next,
	      nextLanes = scheduleTaskForRootDuringMicrotask(root, currentTime);
	    if (0 === nextLanes)
	      (root.next = null),
	        null === prev ? (firstScheduledRoot = next) : (prev.next = next),
	        null === next && (lastScheduledRoot = prev);
	    else if (
	      ((prev = root), 0 !== syncTransitionLanes || 0 !== (nextLanes & 3))
	    )
	      mightHavePendingSyncWork = !0;
	    root = next;
	  }
	  (0 !== pendingEffectsStatus && 5 !== pendingEffectsStatus) ||
	    flushSyncWorkAcrossRoots_impl(syncTransitionLanes, !1);
	  0 !== currentEventTransitionLane && (currentEventTransitionLane = 0);
	}
	function scheduleTaskForRootDuringMicrotask(root, currentTime) {
	  for (
	    var suspendedLanes = root.suspendedLanes,
	      pingedLanes = root.pingedLanes,
	      expirationTimes = root.expirationTimes,
	      lanes = root.pendingLanes & -62914561;
	    0 < lanes;

	  ) {
	    var index$5 = 31 - clz32(lanes),
	      lane = 1 << index$5,
	      expirationTime = expirationTimes[index$5];
	    if (-1 === expirationTime) {
	      if (0 === (lane & suspendedLanes) || 0 !== (lane & pingedLanes))
	        expirationTimes[index$5] = computeExpirationTime(lane, currentTime);
	    } else expirationTime <= currentTime && (root.expiredLanes |= lane);
	    lanes &= ~lane;
	  }
	  currentTime = workInProgressRoot;
	  suspendedLanes = workInProgressRootRenderLanes;
	  suspendedLanes = getNextLanes(
	    root,
	    root === currentTime ? suspendedLanes : 0,
	    null !== root.cancelPendingCommit || -1 !== root.timeoutHandle
	  );
	  pingedLanes = root.callbackNode;
	  if (
	    0 === suspendedLanes ||
	    (root === currentTime &&
	      (2 === workInProgressSuspendedReason ||
	        9 === workInProgressSuspendedReason)) ||
	    null !== root.cancelPendingCommit
	  )
	    return (
	      null !== pingedLanes &&
	        null !== pingedLanes &&
	        cancelCallback$1(pingedLanes),
	      (root.callbackNode = null),
	      (root.callbackPriority = 0)
	    );
	  if (
	    0 === (suspendedLanes & 3) ||
	    checkIfRootIsPrerendering(root, suspendedLanes)
	  ) {
	    currentTime = suspendedLanes & -suspendedLanes;
	    if (currentTime === root.callbackPriority) return currentTime;
	    null !== pingedLanes && cancelCallback$1(pingedLanes);
	    switch (lanesToEventPriority(suspendedLanes)) {
	      case 2:
	      case 8:
	        suspendedLanes = UserBlockingPriority;
	        break;
	      case 32:
	        suspendedLanes = NormalPriority$1;
	        break;
	      case 268435456:
	        suspendedLanes = IdlePriority;
	        break;
	      default:
	        suspendedLanes = NormalPriority$1;
	    }
	    pingedLanes = performWorkOnRootViaSchedulerTask.bind(null, root);
	    suspendedLanes = scheduleCallback$3(suspendedLanes, pingedLanes);
	    root.callbackPriority = currentTime;
	    root.callbackNode = suspendedLanes;
	    return currentTime;
	  }
	  null !== pingedLanes && null !== pingedLanes && cancelCallback$1(pingedLanes);
	  root.callbackPriority = 2;
	  root.callbackNode = null;
	  return 2;
	}
	function performWorkOnRootViaSchedulerTask(root, didTimeout) {
	  if (0 !== pendingEffectsStatus && 5 !== pendingEffectsStatus)
	    return (root.callbackNode = null), (root.callbackPriority = 0), null;
	  var originalCallbackNode = root.callbackNode;
	  if (flushPendingEffects() && root.callbackNode !== originalCallbackNode)
	    return null;
	  var workInProgressRootRenderLanes$jscomp$0 = workInProgressRootRenderLanes;
	  workInProgressRootRenderLanes$jscomp$0 = getNextLanes(
	    root,
	    root === workInProgressRoot ? workInProgressRootRenderLanes$jscomp$0 : 0,
	    null !== root.cancelPendingCommit || -1 !== root.timeoutHandle
	  );
	  if (0 === workInProgressRootRenderLanes$jscomp$0) return null;
	  performWorkOnRoot(root, workInProgressRootRenderLanes$jscomp$0, didTimeout);
	  scheduleTaskForRootDuringMicrotask(root, now());
	  return null != root.callbackNode && root.callbackNode === originalCallbackNode
	    ? performWorkOnRootViaSchedulerTask.bind(null, root)
	    : null;
	}
	function performSyncWorkOnRoot(root, lanes) {
	  if (flushPendingEffects()) return null;
	  performWorkOnRoot(root, lanes, !0);
	}
	function scheduleImmediateRootScheduleTask() {
	  scheduleMicrotask(function () {
	    0 !== (executionContext & 6)
	      ? scheduleCallback$3(
	          ImmediatePriority,
	          processRootScheduleInImmediateTask
	        )
	      : processRootScheduleInMicrotask();
	  });
	}
	function requestTransitionLane() {
	  if (0 === currentEventTransitionLane) {
	    var actionScopeLane = currentEntangledLane;
	    0 === actionScopeLane &&
	      ((actionScopeLane = nextTransitionUpdateLane),
	      (nextTransitionUpdateLane <<= 1),
	      0 === (nextTransitionUpdateLane & 261888) &&
	        (nextTransitionUpdateLane = 256));
	    currentEventTransitionLane = actionScopeLane;
	  }
	  return currentEventTransitionLane;
	}
	function coerceFormActionProp(actionProp) {
	  return null == actionProp ||
	    "symbol" === typeof actionProp ||
	    "boolean" === typeof actionProp
	    ? null
	    : "function" === typeof actionProp
	      ? actionProp
	      : sanitizeURL("" + actionProp);
	}
	function createFormDataWithSubmitter(form, submitter) {
	  var temp = submitter.ownerDocument.createElement("input");
	  temp.name = submitter.name;
	  temp.value = submitter.value;
	  form.id && temp.setAttribute("form", form.id);
	  submitter.parentNode.insertBefore(temp, submitter);
	  form = new FormData(form);
	  temp.parentNode.removeChild(temp);
	  return form;
	}
	function extractEvents$1(
	  dispatchQueue,
	  domEventName,
	  maybeTargetInst,
	  nativeEvent,
	  nativeEventTarget
	) {
	  if (
	    "submit" === domEventName &&
	    maybeTargetInst &&
	    maybeTargetInst.stateNode === nativeEventTarget
	  ) {
	    var action = coerceFormActionProp(
	        (nativeEventTarget[internalPropsKey] || null).action
	      ),
	      submitter = nativeEvent.submitter;
	    submitter &&
	      ((domEventName = (domEventName = submitter[internalPropsKey] || null)
	        ? coerceFormActionProp(domEventName.formAction)
	        : submitter.getAttribute("formAction")),
	      null !== domEventName && ((action = domEventName), (submitter = null)));
	    var event = new SyntheticEvent(
	      "action",
	      "action",
	      null,
	      nativeEvent,
	      nativeEventTarget
	    );
	    dispatchQueue.push({
	      event: event,
	      listeners: [
	        {
	          instance: null,
	          listener: function () {
	            if (nativeEvent.defaultPrevented) {
	              if (0 !== currentEventTransitionLane) {
	                var formData = submitter
	                  ? createFormDataWithSubmitter(nativeEventTarget, submitter)
	                  : new FormData(nativeEventTarget);
	                startHostTransition(
	                  maybeTargetInst,
	                  {
	                    pending: !0,
	                    data: formData,
	                    method: nativeEventTarget.method,
	                    action: action
	                  },
	                  null,
	                  formData
	                );
	              }
	            } else
	              "function" === typeof action &&
	                (event.preventDefault(),
	                (formData = submitter
	                  ? createFormDataWithSubmitter(nativeEventTarget, submitter)
	                  : new FormData(nativeEventTarget)),
	                startHostTransition(
	                  maybeTargetInst,
	                  {
	                    pending: !0,
	                    data: formData,
	                    method: nativeEventTarget.method,
	                    action: action
	                  },
	                  action,
	                  formData
	                ));
	          },
	          currentTarget: nativeEventTarget
	        }
	      ]
	    });
	  }
	}
	for (
	  var i$jscomp$inline_1577 = 0;
	  i$jscomp$inline_1577 < simpleEventPluginEvents.length;
	  i$jscomp$inline_1577++
	) {
	  var eventName$jscomp$inline_1578 =
	      simpleEventPluginEvents[i$jscomp$inline_1577],
	    domEventName$jscomp$inline_1579 =
	      eventName$jscomp$inline_1578.toLowerCase(),
	    capitalizedEvent$jscomp$inline_1580 =
	      eventName$jscomp$inline_1578[0].toUpperCase() +
	      eventName$jscomp$inline_1578.slice(1);
	  registerSimpleEvent(
	    domEventName$jscomp$inline_1579,
	    "on" + capitalizedEvent$jscomp$inline_1580
	  );
	}
	registerSimpleEvent(ANIMATION_END, "onAnimationEnd");
	registerSimpleEvent(ANIMATION_ITERATION, "onAnimationIteration");
	registerSimpleEvent(ANIMATION_START, "onAnimationStart");
	registerSimpleEvent("dblclick", "onDoubleClick");
	registerSimpleEvent("focusin", "onFocus");
	registerSimpleEvent("focusout", "onBlur");
	registerSimpleEvent(TRANSITION_RUN, "onTransitionRun");
	registerSimpleEvent(TRANSITION_START, "onTransitionStart");
	registerSimpleEvent(TRANSITION_CANCEL, "onTransitionCancel");
	registerSimpleEvent(TRANSITION_END, "onTransitionEnd");
	registerDirectEvent("onMouseEnter", ["mouseout", "mouseover"]);
	registerDirectEvent("onMouseLeave", ["mouseout", "mouseover"]);
	registerDirectEvent("onPointerEnter", ["pointerout", "pointerover"]);
	registerDirectEvent("onPointerLeave", ["pointerout", "pointerover"]);
	registerTwoPhaseEvent(
	  "onChange",
	  "change click focusin focusout input keydown keyup selectionchange".split(" ")
	);
	registerTwoPhaseEvent(
	  "onSelect",
	  "focusout contextmenu dragend focusin keydown keyup mousedown mouseup selectionchange".split(
	    " "
	  )
	);
	registerTwoPhaseEvent("onBeforeInput", [
	  "compositionend",
	  "keypress",
	  "textInput",
	  "paste"
	]);
	registerTwoPhaseEvent(
	  "onCompositionEnd",
	  "compositionend focusout keydown keypress keyup mousedown".split(" ")
	);
	registerTwoPhaseEvent(
	  "onCompositionStart",
	  "compositionstart focusout keydown keypress keyup mousedown".split(" ")
	);
	registerTwoPhaseEvent(
	  "onCompositionUpdate",
	  "compositionupdate focusout keydown keypress keyup mousedown".split(" ")
	);
	var mediaEventTypes =
	    "abort canplay canplaythrough durationchange emptied encrypted ended error loadeddata loadedmetadata loadstart pause play playing progress ratechange resize seeked seeking stalled suspend timeupdate volumechange waiting".split(
	      " "
	    ),
	  nonDelegatedEvents = new Set(
	    "beforetoggle cancel close invalid load scroll scrollend toggle"
	      .split(" ")
	      .concat(mediaEventTypes)
	  );
	function processDispatchQueue(dispatchQueue, eventSystemFlags) {
	  eventSystemFlags = 0 !== (eventSystemFlags & 4);
	  for (var i = 0; i < dispatchQueue.length; i++) {
	    var _dispatchQueue$i = dispatchQueue[i],
	      event = _dispatchQueue$i.event;
	    _dispatchQueue$i = _dispatchQueue$i.listeners;
	    a: {
	      var previousInstance = void 0;
	      if (eventSystemFlags)
	        for (
	          var i$jscomp$0 = _dispatchQueue$i.length - 1;
	          0 <= i$jscomp$0;
	          i$jscomp$0--
	        ) {
	          var _dispatchListeners$i = _dispatchQueue$i[i$jscomp$0],
	            instance = _dispatchListeners$i.instance,
	            currentTarget = _dispatchListeners$i.currentTarget;
	          _dispatchListeners$i = _dispatchListeners$i.listener;
	          if (instance !== previousInstance && event.isPropagationStopped())
	            break a;
	          previousInstance = _dispatchListeners$i;
	          event.currentTarget = currentTarget;
	          try {
	            previousInstance(event);
	          } catch (error) {
	            reportGlobalError(error);
	          }
	          event.currentTarget = null;
	          previousInstance = instance;
	        }
	      else
	        for (
	          i$jscomp$0 = 0;
	          i$jscomp$0 < _dispatchQueue$i.length;
	          i$jscomp$0++
	        ) {
	          _dispatchListeners$i = _dispatchQueue$i[i$jscomp$0];
	          instance = _dispatchListeners$i.instance;
	          currentTarget = _dispatchListeners$i.currentTarget;
	          _dispatchListeners$i = _dispatchListeners$i.listener;
	          if (instance !== previousInstance && event.isPropagationStopped())
	            break a;
	          previousInstance = _dispatchListeners$i;
	          event.currentTarget = currentTarget;
	          try {
	            previousInstance(event);
	          } catch (error) {
	            reportGlobalError(error);
	          }
	          event.currentTarget = null;
	          previousInstance = instance;
	        }
	    }
	  }
	}
	function listenToNonDelegatedEvent(domEventName, targetElement) {
	  var JSCompiler_inline_result = targetElement[internalEventHandlersKey];
	  void 0 === JSCompiler_inline_result &&
	    (JSCompiler_inline_result = targetElement[internalEventHandlersKey] =
	      new Set());
	  var listenerSetKey = domEventName + "__bubble";
	  JSCompiler_inline_result.has(listenerSetKey) ||
	    (addTrappedEventListener(targetElement, domEventName, 2, !1),
	    JSCompiler_inline_result.add(listenerSetKey));
	}
	function listenToNativeEvent(domEventName, isCapturePhaseListener, target) {
	  var eventSystemFlags = 0;
	  isCapturePhaseListener && (eventSystemFlags |= 4);
	  addTrappedEventListener(
	    target,
	    domEventName,
	    eventSystemFlags,
	    isCapturePhaseListener
	  );
	}
	var listeningMarker = "_reactListening" + Math.random().toString(36).slice(2);
	function listenToAllSupportedEvents(rootContainerElement) {
	  if (!rootContainerElement[listeningMarker]) {
	    rootContainerElement[listeningMarker] = !0;
	    allNativeEvents.forEach(function (domEventName) {
	      "selectionchange" !== domEventName &&
	        (nonDelegatedEvents.has(domEventName) ||
	          listenToNativeEvent(domEventName, !1, rootContainerElement),
	        listenToNativeEvent(domEventName, !0, rootContainerElement));
	    });
	    var ownerDocument =
	      9 === rootContainerElement.nodeType
	        ? rootContainerElement
	        : rootContainerElement.ownerDocument;
	    null === ownerDocument ||
	      ownerDocument[listeningMarker] ||
	      ((ownerDocument[listeningMarker] = !0),
	      listenToNativeEvent("selectionchange", !1, ownerDocument));
	  }
	}
	function addTrappedEventListener(
	  targetContainer,
	  domEventName,
	  eventSystemFlags,
	  isCapturePhaseListener
	) {
	  switch (getEventPriority(domEventName)) {
	    case 2:
	      var listenerWrapper = dispatchDiscreteEvent;
	      break;
	    case 8:
	      listenerWrapper = dispatchContinuousEvent;
	      break;
	    default:
	      listenerWrapper = dispatchEvent;
	  }
	  eventSystemFlags = listenerWrapper.bind(
	    null,
	    domEventName,
	    eventSystemFlags,
	    targetContainer
	  );
	  listenerWrapper = void 0;
	  !passiveBrowserEventsSupported ||
	    ("touchstart" !== domEventName &&
	      "touchmove" !== domEventName &&
	      "wheel" !== domEventName) ||
	    (listenerWrapper = !0);
	  isCapturePhaseListener
	    ? void 0 !== listenerWrapper
	      ? targetContainer.addEventListener(domEventName, eventSystemFlags, {
	          capture: !0,
	          passive: listenerWrapper
	        })
	      : targetContainer.addEventListener(domEventName, eventSystemFlags, !0)
	    : void 0 !== listenerWrapper
	      ? targetContainer.addEventListener(domEventName, eventSystemFlags, {
	          passive: listenerWrapper
	        })
	      : targetContainer.addEventListener(domEventName, eventSystemFlags, !1);
	}
	function dispatchEventForPluginEventSystem(
	  domEventName,
	  eventSystemFlags,
	  nativeEvent,
	  targetInst$jscomp$0,
	  targetContainer
	) {
	  var ancestorInst = targetInst$jscomp$0;
	  if (
	    0 === (eventSystemFlags & 1) &&
	    0 === (eventSystemFlags & 2) &&
	    null !== targetInst$jscomp$0
	  )
	    a: for (;;) {
	      if (null === targetInst$jscomp$0) return;
	      var nodeTag = targetInst$jscomp$0.tag;
	      if (3 === nodeTag || 4 === nodeTag) {
	        var container = targetInst$jscomp$0.stateNode.containerInfo;
	        if (container === targetContainer) break;
	        if (4 === nodeTag)
	          for (nodeTag = targetInst$jscomp$0.return; null !== nodeTag; ) {
	            var grandTag = nodeTag.tag;
	            if (
	              (3 === grandTag || 4 === grandTag) &&
	              nodeTag.stateNode.containerInfo === targetContainer
	            )
	              return;
	            nodeTag = nodeTag.return;
	          }
	        for (; null !== container; ) {
	          nodeTag = getClosestInstanceFromNode(container);
	          if (null === nodeTag) return;
	          grandTag = nodeTag.tag;
	          if (
	            5 === grandTag ||
	            6 === grandTag ||
	            26 === grandTag ||
	            27 === grandTag
	          ) {
	            targetInst$jscomp$0 = ancestorInst = nodeTag;
	            continue a;
	          }
	          container = container.parentNode;
	        }
	      }
	      targetInst$jscomp$0 = targetInst$jscomp$0.return;
	    }
	  batchedUpdates$1(function () {
	    var targetInst = ancestorInst,
	      nativeEventTarget = getEventTarget(nativeEvent),
	      dispatchQueue = [];
	    a: {
	      var reactName = topLevelEventsToReactNames.get(domEventName);
	      if (void 0 !== reactName) {
	        var SyntheticEventCtor = SyntheticEvent,
	          reactEventType = domEventName;
	        switch (domEventName) {
	          case "keypress":
	            if (0 === getEventCharCode(nativeEvent)) break a;
	          case "keydown":
	          case "keyup":
	            SyntheticEventCtor = SyntheticKeyboardEvent;
	            break;
	          case "focusin":
	            reactEventType = "focus";
	            SyntheticEventCtor = SyntheticFocusEvent;
	            break;
	          case "focusout":
	            reactEventType = "blur";
	            SyntheticEventCtor = SyntheticFocusEvent;
	            break;
	          case "beforeblur":
	          case "afterblur":
	            SyntheticEventCtor = SyntheticFocusEvent;
	            break;
	          case "click":
	            if (2 === nativeEvent.button) break a;
	          case "auxclick":
	          case "dblclick":
	          case "mousedown":
	          case "mousemove":
	          case "mouseup":
	          case "mouseout":
	          case "mouseover":
	          case "contextmenu":
	            SyntheticEventCtor = SyntheticMouseEvent;
	            break;
	          case "drag":
	          case "dragend":
	          case "dragenter":
	          case "dragexit":
	          case "dragleave":
	          case "dragover":
	          case "dragstart":
	          case "drop":
	            SyntheticEventCtor = SyntheticDragEvent;
	            break;
	          case "touchcancel":
	          case "touchend":
	          case "touchmove":
	          case "touchstart":
	            SyntheticEventCtor = SyntheticTouchEvent;
	            break;
	          case ANIMATION_END:
	          case ANIMATION_ITERATION:
	          case ANIMATION_START:
	            SyntheticEventCtor = SyntheticAnimationEvent;
	            break;
	          case TRANSITION_END:
	            SyntheticEventCtor = SyntheticTransitionEvent;
	            break;
	          case "scroll":
	          case "scrollend":
	            SyntheticEventCtor = SyntheticUIEvent;
	            break;
	          case "wheel":
	            SyntheticEventCtor = SyntheticWheelEvent;
	            break;
	          case "copy":
	          case "cut":
	          case "paste":
	            SyntheticEventCtor = SyntheticClipboardEvent;
	            break;
	          case "gotpointercapture":
	          case "lostpointercapture":
	          case "pointercancel":
	          case "pointerdown":
	          case "pointermove":
	          case "pointerout":
	          case "pointerover":
	          case "pointerup":
	            SyntheticEventCtor = SyntheticPointerEvent;
	            break;
	          case "toggle":
	          case "beforetoggle":
	            SyntheticEventCtor = SyntheticToggleEvent;
	        }
	        var inCapturePhase = 0 !== (eventSystemFlags & 4),
	          accumulateTargetOnly =
	            !inCapturePhase &&
	            ("scroll" === domEventName || "scrollend" === domEventName),
	          reactEventName = inCapturePhase
	            ? null !== reactName
	              ? reactName + "Capture"
	              : null
	            : reactName;
	        inCapturePhase = [];
	        for (
	          var instance = targetInst, lastHostComponent;
	          null !== instance;

	        ) {
	          var _instance = instance;
	          lastHostComponent = _instance.stateNode;
	          _instance = _instance.tag;
	          (5 !== _instance && 26 !== _instance && 27 !== _instance) ||
	            null === lastHostComponent ||
	            null === reactEventName ||
	            ((_instance = getListener(instance, reactEventName)),
	            null != _instance &&
	              inCapturePhase.push(
	                createDispatchListener(instance, _instance, lastHostComponent)
	              ));
	          if (accumulateTargetOnly) break;
	          instance = instance.return;
	        }
	        0 < inCapturePhase.length &&
	          ((reactName = new SyntheticEventCtor(
	            reactName,
	            reactEventType,
	            null,
	            nativeEvent,
	            nativeEventTarget
	          )),
	          dispatchQueue.push({ event: reactName, listeners: inCapturePhase }));
	      }
	    }
	    if (0 === (eventSystemFlags & 7)) {
	      a: {
	        reactName =
	          "mouseover" === domEventName || "pointerover" === domEventName;
	        SyntheticEventCtor =
	          "mouseout" === domEventName || "pointerout" === domEventName;
	        if (
	          reactName &&
	          nativeEvent !== currentReplayingEvent &&
	          (reactEventType =
	            nativeEvent.relatedTarget || nativeEvent.fromElement) &&
	          (getClosestInstanceFromNode(reactEventType) ||
	            reactEventType[internalContainerInstanceKey])
	        )
	          break a;
	        if (SyntheticEventCtor || reactName) {
	          reactName =
	            nativeEventTarget.window === nativeEventTarget
	              ? nativeEventTarget
	              : (reactName = nativeEventTarget.ownerDocument)
	                ? reactName.defaultView || reactName.parentWindow
	                : window;
	          if (SyntheticEventCtor) {
	            if (
	              ((reactEventType =
	                nativeEvent.relatedTarget || nativeEvent.toElement),
	              (SyntheticEventCtor = targetInst),
	              (reactEventType = reactEventType
	                ? getClosestInstanceFromNode(reactEventType)
	                : null),
	              null !== reactEventType &&
	                ((accumulateTargetOnly =
	                  getNearestMountedFiber(reactEventType)),
	                (inCapturePhase = reactEventType.tag),
	                reactEventType !== accumulateTargetOnly ||
	                  (5 !== inCapturePhase &&
	                    27 !== inCapturePhase &&
	                    6 !== inCapturePhase)))
	            )
	              reactEventType = null;
	          } else (SyntheticEventCtor = null), (reactEventType = targetInst);
	          if (SyntheticEventCtor !== reactEventType) {
	            inCapturePhase = SyntheticMouseEvent;
	            _instance = "onMouseLeave";
	            reactEventName = "onMouseEnter";
	            instance = "mouse";
	            if ("pointerout" === domEventName || "pointerover" === domEventName)
	              (inCapturePhase = SyntheticPointerEvent),
	                (_instance = "onPointerLeave"),
	                (reactEventName = "onPointerEnter"),
	                (instance = "pointer");
	            accumulateTargetOnly =
	              null == SyntheticEventCtor
	                ? reactName
	                : getNodeFromInstance(SyntheticEventCtor);
	            lastHostComponent =
	              null == reactEventType
	                ? reactName
	                : getNodeFromInstance(reactEventType);
	            reactName = new inCapturePhase(
	              _instance,
	              instance + "leave",
	              SyntheticEventCtor,
	              nativeEvent,
	              nativeEventTarget
	            );
	            reactName.target = accumulateTargetOnly;
	            reactName.relatedTarget = lastHostComponent;
	            _instance = null;
	            getClosestInstanceFromNode(nativeEventTarget) === targetInst &&
	              ((inCapturePhase = new inCapturePhase(
	                reactEventName,
	                instance + "enter",
	                reactEventType,
	                nativeEvent,
	                nativeEventTarget
	              )),
	              (inCapturePhase.target = lastHostComponent),
	              (inCapturePhase.relatedTarget = accumulateTargetOnly),
	              (_instance = inCapturePhase));
	            accumulateTargetOnly = _instance;
	            if (SyntheticEventCtor && reactEventType)
	              b: {
	                inCapturePhase = getParent;
	                reactEventName = SyntheticEventCtor;
	                instance = reactEventType;
	                lastHostComponent = 0;
	                for (
	                  _instance = reactEventName;
	                  _instance;
	                  _instance = inCapturePhase(_instance)
	                )
	                  lastHostComponent++;
	                _instance = 0;
	                for (var tempB = instance; tempB; tempB = inCapturePhase(tempB))
	                  _instance++;
	                for (; 0 < lastHostComponent - _instance; )
	                  (reactEventName = inCapturePhase(reactEventName)),
	                    lastHostComponent--;
	                for (; 0 < _instance - lastHostComponent; )
	                  (instance = inCapturePhase(instance)), _instance--;
	                for (; lastHostComponent--; ) {
	                  if (
	                    reactEventName === instance ||
	                    (null !== instance && reactEventName === instance.alternate)
	                  ) {
	                    inCapturePhase = reactEventName;
	                    break b;
	                  }
	                  reactEventName = inCapturePhase(reactEventName);
	                  instance = inCapturePhase(instance);
	                }
	                inCapturePhase = null;
	              }
	            else inCapturePhase = null;
	            null !== SyntheticEventCtor &&
	              accumulateEnterLeaveListenersForEvent(
	                dispatchQueue,
	                reactName,
	                SyntheticEventCtor,
	                inCapturePhase,
	                !1
	              );
	            null !== reactEventType &&
	              null !== accumulateTargetOnly &&
	              accumulateEnterLeaveListenersForEvent(
	                dispatchQueue,
	                accumulateTargetOnly,
	                reactEventType,
	                inCapturePhase,
	                !0
	              );
	          }
	        }
	      }
	      a: {
	        reactName = targetInst ? getNodeFromInstance(targetInst) : window;
	        SyntheticEventCtor =
	          reactName.nodeName && reactName.nodeName.toLowerCase();
	        if (
	          "select" === SyntheticEventCtor ||
	          ("input" === SyntheticEventCtor && "file" === reactName.type)
	        )
	          var getTargetInstFunc = getTargetInstForChangeEvent;
	        else if (isTextInputElement(reactName))
	          if (isInputEventSupported)
	            getTargetInstFunc = getTargetInstForInputOrChangeEvent;
	          else {
	            getTargetInstFunc = getTargetInstForInputEventPolyfill;
	            var handleEventFunc = handleEventsForInputEventPolyfill;
	          }
	        else
	          (SyntheticEventCtor = reactName.nodeName),
	            !SyntheticEventCtor ||
	            "input" !== SyntheticEventCtor.toLowerCase() ||
	            ("checkbox" !== reactName.type && "radio" !== reactName.type)
	              ? targetInst &&
	                isCustomElement(targetInst.elementType) &&
	                (getTargetInstFunc = getTargetInstForChangeEvent)
	              : (getTargetInstFunc = getTargetInstForClickEvent);
	        if (
	          getTargetInstFunc &&
	          (getTargetInstFunc = getTargetInstFunc(domEventName, targetInst))
	        ) {
	          createAndAccumulateChangeEvent(
	            dispatchQueue,
	            getTargetInstFunc,
	            nativeEvent,
	            nativeEventTarget
	          );
	          break a;
	        }
	        handleEventFunc && handleEventFunc(domEventName, reactName, targetInst);
	        "focusout" === domEventName &&
	          targetInst &&
	          "number" === reactName.type &&
	          null != targetInst.memoizedProps.value &&
	          setDefaultValue(reactName, "number", reactName.value);
	      }
	      handleEventFunc = targetInst ? getNodeFromInstance(targetInst) : window;
	      switch (domEventName) {
	        case "focusin":
	          if (
	            isTextInputElement(handleEventFunc) ||
	            "true" === handleEventFunc.contentEditable
	          )
	            (activeElement = handleEventFunc),
	              (activeElementInst = targetInst),
	              (lastSelection = null);
	          break;
	        case "focusout":
	          lastSelection = activeElementInst = activeElement = null;
	          break;
	        case "mousedown":
	          mouseDown = !0;
	          break;
	        case "contextmenu":
	        case "mouseup":
	        case "dragend":
	          mouseDown = !1;
	          constructSelectEvent(dispatchQueue, nativeEvent, nativeEventTarget);
	          break;
	        case "selectionchange":
	          if (skipSelectionChangeEvent) break;
	        case "keydown":
	        case "keyup":
	          constructSelectEvent(dispatchQueue, nativeEvent, nativeEventTarget);
	      }
	      var fallbackData;
	      if (canUseCompositionEvent)
	        b: {
	          switch (domEventName) {
	            case "compositionstart":
	              var eventType = "onCompositionStart";
	              break b;
	            case "compositionend":
	              eventType = "onCompositionEnd";
	              break b;
	            case "compositionupdate":
	              eventType = "onCompositionUpdate";
	              break b;
	          }
	          eventType = void 0;
	        }
	      else
	        isComposing
	          ? isFallbackCompositionEnd(domEventName, nativeEvent) &&
	            (eventType = "onCompositionEnd")
	          : "keydown" === domEventName &&
	            229 === nativeEvent.keyCode &&
	            (eventType = "onCompositionStart");
	      eventType &&
	        (useFallbackCompositionData &&
	          "ko" !== nativeEvent.locale &&
	          (isComposing || "onCompositionStart" !== eventType
	            ? "onCompositionEnd" === eventType &&
	              isComposing &&
	              (fallbackData = getData())
	            : ((root = nativeEventTarget),
	              (startText = "value" in root ? root.value : root.textContent),
	              (isComposing = !0))),
	        (handleEventFunc = accumulateTwoPhaseListeners(targetInst, eventType)),
	        0 < handleEventFunc.length &&
	          ((eventType = new SyntheticCompositionEvent(
	            eventType,
	            domEventName,
	            null,
	            nativeEvent,
	            nativeEventTarget
	          )),
	          dispatchQueue.push({ event: eventType, listeners: handleEventFunc }),
	          fallbackData
	            ? (eventType.data = fallbackData)
	            : ((fallbackData = getDataFromCustomEvent(nativeEvent)),
	              null !== fallbackData && (eventType.data = fallbackData))));
	      if (
	        (fallbackData = canUseTextInputEvent
	          ? getNativeBeforeInputChars(domEventName, nativeEvent)
	          : getFallbackBeforeInputChars(domEventName, nativeEvent))
	      )
	        (eventType = accumulateTwoPhaseListeners(targetInst, "onBeforeInput")),
	          0 < eventType.length &&
	            ((handleEventFunc = new SyntheticCompositionEvent(
	              "onBeforeInput",
	              "beforeinput",
	              null,
	              nativeEvent,
	              nativeEventTarget
	            )),
	            dispatchQueue.push({
	              event: handleEventFunc,
	              listeners: eventType
	            }),
	            (handleEventFunc.data = fallbackData));
	      extractEvents$1(
	        dispatchQueue,
	        domEventName,
	        targetInst,
	        nativeEvent,
	        nativeEventTarget
	      );
	    }
	    processDispatchQueue(dispatchQueue, eventSystemFlags);
	  });
	}
	function createDispatchListener(instance, listener, currentTarget) {
	  return {
	    instance: instance,
	    listener: listener,
	    currentTarget: currentTarget
	  };
	}
	function accumulateTwoPhaseListeners(targetFiber, reactName) {
	  for (
	    var captureName = reactName + "Capture", listeners = [];
	    null !== targetFiber;

	  ) {
	    var _instance2 = targetFiber,
	      stateNode = _instance2.stateNode;
	    _instance2 = _instance2.tag;
	    (5 !== _instance2 && 26 !== _instance2 && 27 !== _instance2) ||
	      null === stateNode ||
	      ((_instance2 = getListener(targetFiber, captureName)),
	      null != _instance2 &&
	        listeners.unshift(
	          createDispatchListener(targetFiber, _instance2, stateNode)
	        ),
	      (_instance2 = getListener(targetFiber, reactName)),
	      null != _instance2 &&
	        listeners.push(
	          createDispatchListener(targetFiber, _instance2, stateNode)
	        ));
	    if (3 === targetFiber.tag) return listeners;
	    targetFiber = targetFiber.return;
	  }
	  return [];
	}
	function getParent(inst) {
	  if (null === inst) return null;
	  do inst = inst.return;
	  while (inst && 5 !== inst.tag && 27 !== inst.tag);
	  return inst ? inst : null;
	}
	function accumulateEnterLeaveListenersForEvent(
	  dispatchQueue,
	  event,
	  target,
	  common,
	  inCapturePhase
	) {
	  for (
	    var registrationName = event._reactName, listeners = [];
	    null !== target && target !== common;

	  ) {
	    var _instance3 = target,
	      alternate = _instance3.alternate,
	      stateNode = _instance3.stateNode;
	    _instance3 = _instance3.tag;
	    if (null !== alternate && alternate === common) break;
	    (5 !== _instance3 && 26 !== _instance3 && 27 !== _instance3) ||
	      null === stateNode ||
	      ((alternate = stateNode),
	      inCapturePhase
	        ? ((stateNode = getListener(target, registrationName)),
	          null != stateNode &&
	            listeners.unshift(
	              createDispatchListener(target, stateNode, alternate)
	            ))
	        : inCapturePhase ||
	          ((stateNode = getListener(target, registrationName)),
	          null != stateNode &&
	            listeners.push(
	              createDispatchListener(target, stateNode, alternate)
	            )));
	    target = target.return;
	  }
	  0 !== listeners.length &&
	    dispatchQueue.push({ event: event, listeners: listeners });
	}
	var NORMALIZE_NEWLINES_REGEX = /\r\n?/g,
	  NORMALIZE_NULL_AND_REPLACEMENT_REGEX = /\u0000|\uFFFD/g;
	function normalizeMarkupForTextOrAttribute(markup) {
	  return ("string" === typeof markup ? markup : "" + markup)
	    .replace(NORMALIZE_NEWLINES_REGEX, "\n")
	    .replace(NORMALIZE_NULL_AND_REPLACEMENT_REGEX, "");
	}
	function checkForUnmatchedText(serverText, clientText) {
	  clientText = normalizeMarkupForTextOrAttribute(clientText);
	  return normalizeMarkupForTextOrAttribute(serverText) === clientText ? !0 : !1;
	}
	function setProp(domElement, tag, key, value, props, prevValue) {
	  switch (key) {
	    case "children":
	      "string" === typeof value
	        ? "body" === tag ||
	          ("textarea" === tag && "" === value) ||
	          setTextContent(domElement, value)
	        : ("number" === typeof value || "bigint" === typeof value) &&
	          "body" !== tag &&
	          setTextContent(domElement, "" + value);
	      break;
	    case "className":
	      setValueForKnownAttribute(domElement, "class", value);
	      break;
	    case "tabIndex":
	      setValueForKnownAttribute(domElement, "tabindex", value);
	      break;
	    case "dir":
	    case "role":
	    case "viewBox":
	    case "width":
	    case "height":
	      setValueForKnownAttribute(domElement, key, value);
	      break;
	    case "style":
	      setValueForStyles(domElement, value, prevValue);
	      break;
	    case "data":
	      if ("object" !== tag) {
	        setValueForKnownAttribute(domElement, "data", value);
	        break;
	      }
	    case "src":
	    case "href":
	      if ("" === value && ("a" !== tag || "href" !== key)) {
	        domElement.removeAttribute(key);
	        break;
	      }
	      if (
	        null == value ||
	        "function" === typeof value ||
	        "symbol" === typeof value ||
	        "boolean" === typeof value
	      ) {
	        domElement.removeAttribute(key);
	        break;
	      }
	      value = sanitizeURL("" + value);
	      domElement.setAttribute(key, value);
	      break;
	    case "action":
	    case "formAction":
	      if ("function" === typeof value) {
	        domElement.setAttribute(
	          key,
	          "javascript:throw new Error('A React form was unexpectedly submitted. If you called form.submit() manually, consider using form.requestSubmit() instead. If you\\'re trying to use event.stopPropagation() in a submit event handler, consider also calling event.preventDefault().')"
	        );
	        break;
	      } else
	        "function" === typeof prevValue &&
	          ("formAction" === key
	            ? ("input" !== tag &&
	                setProp(domElement, tag, "name", props.name, props, null),
	              setProp(
	                domElement,
	                tag,
	                "formEncType",
	                props.formEncType,
	                props,
	                null
	              ),
	              setProp(
	                domElement,
	                tag,
	                "formMethod",
	                props.formMethod,
	                props,
	                null
	              ),
	              setProp(
	                domElement,
	                tag,
	                "formTarget",
	                props.formTarget,
	                props,
	                null
	              ))
	            : (setProp(domElement, tag, "encType", props.encType, props, null),
	              setProp(domElement, tag, "method", props.method, props, null),
	              setProp(domElement, tag, "target", props.target, props, null)));
	      if (
	        null == value ||
	        "symbol" === typeof value ||
	        "boolean" === typeof value
	      ) {
	        domElement.removeAttribute(key);
	        break;
	      }
	      value = sanitizeURL("" + value);
	      domElement.setAttribute(key, value);
	      break;
	    case "onClick":
	      null != value && (domElement.onclick = noop$1);
	      break;
	    case "onScroll":
	      null != value && listenToNonDelegatedEvent("scroll", domElement);
	      break;
	    case "onScrollEnd":
	      null != value && listenToNonDelegatedEvent("scrollend", domElement);
	      break;
	    case "dangerouslySetInnerHTML":
	      if (null != value) {
	        if ("object" !== typeof value || !("__html" in value))
	          throw Error(formatProdErrorMessage(61));
	        key = value.__html;
	        if (null != key) {
	          if (null != props.children) throw Error(formatProdErrorMessage(60));
	          domElement.innerHTML = key;
	        }
	      }
	      break;
	    case "multiple":
	      domElement.multiple =
	        value && "function" !== typeof value && "symbol" !== typeof value;
	      break;
	    case "muted":
	      domElement.muted =
	        value && "function" !== typeof value && "symbol" !== typeof value;
	      break;
	    case "suppressContentEditableWarning":
	    case "suppressHydrationWarning":
	    case "defaultValue":
	    case "defaultChecked":
	    case "innerHTML":
	    case "ref":
	      break;
	    case "autoFocus":
	      break;
	    case "xlinkHref":
	      if (
	        null == value ||
	        "function" === typeof value ||
	        "boolean" === typeof value ||
	        "symbol" === typeof value
	      ) {
	        domElement.removeAttribute("xlink:href");
	        break;
	      }
	      key = sanitizeURL("" + value);
	      domElement.setAttributeNS(
	        "http://www.w3.org/1999/xlink",
	        "xlink:href",
	        key
	      );
	      break;
	    case "contentEditable":
	    case "spellCheck":
	    case "draggable":
	    case "value":
	    case "autoReverse":
	    case "externalResourcesRequired":
	    case "focusable":
	    case "preserveAlpha":
	      null != value && "function" !== typeof value && "symbol" !== typeof value
	        ? domElement.setAttribute(key, "" + value)
	        : domElement.removeAttribute(key);
	      break;
	    case "inert":
	    case "allowFullScreen":
	    case "async":
	    case "autoPlay":
	    case "controls":
	    case "default":
	    case "defer":
	    case "disabled":
	    case "disablePictureInPicture":
	    case "disableRemotePlayback":
	    case "formNoValidate":
	    case "hidden":
	    case "loop":
	    case "noModule":
	    case "noValidate":
	    case "open":
	    case "playsInline":
	    case "readOnly":
	    case "required":
	    case "reversed":
	    case "scoped":
	    case "seamless":
	    case "itemScope":
	      value && "function" !== typeof value && "symbol" !== typeof value
	        ? domElement.setAttribute(key, "")
	        : domElement.removeAttribute(key);
	      break;
	    case "capture":
	    case "download":
	      !0 === value
	        ? domElement.setAttribute(key, "")
	        : !1 !== value &&
	            null != value &&
	            "function" !== typeof value &&
	            "symbol" !== typeof value
	          ? domElement.setAttribute(key, value)
	          : domElement.removeAttribute(key);
	      break;
	    case "cols":
	    case "rows":
	    case "size":
	    case "span":
	      null != value &&
	      "function" !== typeof value &&
	      "symbol" !== typeof value &&
	      !isNaN(value) &&
	      1 <= value
	        ? domElement.setAttribute(key, value)
	        : domElement.removeAttribute(key);
	      break;
	    case "rowSpan":
	    case "start":
	      null == value ||
	      "function" === typeof value ||
	      "symbol" === typeof value ||
	      isNaN(value)
	        ? domElement.removeAttribute(key)
	        : domElement.setAttribute(key, value);
	      break;
	    case "popover":
	      listenToNonDelegatedEvent("beforetoggle", domElement);
	      listenToNonDelegatedEvent("toggle", domElement);
	      setValueForAttribute(domElement, "popover", value);
	      break;
	    case "xlinkActuate":
	      setValueForNamespacedAttribute(
	        domElement,
	        "http://www.w3.org/1999/xlink",
	        "xlink:actuate",
	        value
	      );
	      break;
	    case "xlinkArcrole":
	      setValueForNamespacedAttribute(
	        domElement,
	        "http://www.w3.org/1999/xlink",
	        "xlink:arcrole",
	        value
	      );
	      break;
	    case "xlinkRole":
	      setValueForNamespacedAttribute(
	        domElement,
	        "http://www.w3.org/1999/xlink",
	        "xlink:role",
	        value
	      );
	      break;
	    case "xlinkShow":
	      setValueForNamespacedAttribute(
	        domElement,
	        "http://www.w3.org/1999/xlink",
	        "xlink:show",
	        value
	      );
	      break;
	    case "xlinkTitle":
	      setValueForNamespacedAttribute(
	        domElement,
	        "http://www.w3.org/1999/xlink",
	        "xlink:title",
	        value
	      );
	      break;
	    case "xlinkType":
	      setValueForNamespacedAttribute(
	        domElement,
	        "http://www.w3.org/1999/xlink",
	        "xlink:type",
	        value
	      );
	      break;
	    case "xmlBase":
	      setValueForNamespacedAttribute(
	        domElement,
	        "http://www.w3.org/XML/1998/namespace",
	        "xml:base",
	        value
	      );
	      break;
	    case "xmlLang":
	      setValueForNamespacedAttribute(
	        domElement,
	        "http://www.w3.org/XML/1998/namespace",
	        "xml:lang",
	        value
	      );
	      break;
	    case "xmlSpace":
	      setValueForNamespacedAttribute(
	        domElement,
	        "http://www.w3.org/XML/1998/namespace",
	        "xml:space",
	        value
	      );
	      break;
	    case "is":
	      setValueForAttribute(domElement, "is", value);
	      break;
	    case "innerText":
	    case "textContent":
	      break;
	    default:
	      if (
	        !(2 < key.length) ||
	        ("o" !== key[0] && "O" !== key[0]) ||
	        ("n" !== key[1] && "N" !== key[1])
	      )
	        (key = aliases.get(key) || key),
	          setValueForAttribute(domElement, key, value);
	  }
	}
	function setPropOnCustomElement(domElement, tag, key, value, props, prevValue) {
	  switch (key) {
	    case "style":
	      setValueForStyles(domElement, value, prevValue);
	      break;
	    case "dangerouslySetInnerHTML":
	      if (null != value) {
	        if ("object" !== typeof value || !("__html" in value))
	          throw Error(formatProdErrorMessage(61));
	        key = value.__html;
	        if (null != key) {
	          if (null != props.children) throw Error(formatProdErrorMessage(60));
	          domElement.innerHTML = key;
	        }
	      }
	      break;
	    case "children":
	      "string" === typeof value
	        ? setTextContent(domElement, value)
	        : ("number" === typeof value || "bigint" === typeof value) &&
	          setTextContent(domElement, "" + value);
	      break;
	    case "onScroll":
	      null != value && listenToNonDelegatedEvent("scroll", domElement);
	      break;
	    case "onScrollEnd":
	      null != value && listenToNonDelegatedEvent("scrollend", domElement);
	      break;
	    case "onClick":
	      null != value && (domElement.onclick = noop$1);
	      break;
	    case "suppressContentEditableWarning":
	    case "suppressHydrationWarning":
	    case "innerHTML":
	    case "ref":
	      break;
	    case "innerText":
	    case "textContent":
	      break;
	    default:
	      if (!registrationNameDependencies.hasOwnProperty(key))
	        a: {
	          if (
	            "o" === key[0] &&
	            "n" === key[1] &&
	            ((props = key.endsWith("Capture")),
	            (tag = key.slice(2, props ? key.length - 7 : void 0)),
	            (prevValue = domElement[internalPropsKey] || null),
	            (prevValue = null != prevValue ? prevValue[key] : null),
	            "function" === typeof prevValue &&
	              domElement.removeEventListener(tag, prevValue, props),
	            "function" === typeof value)
	          ) {
	            "function" !== typeof prevValue &&
	              null !== prevValue &&
	              (key in domElement
	                ? (domElement[key] = null)
	                : domElement.hasAttribute(key) &&
	                  domElement.removeAttribute(key));
	            domElement.addEventListener(tag, value, props);
	            break a;
	          }
	          key in domElement
	            ? (domElement[key] = value)
	            : !0 === value
	              ? domElement.setAttribute(key, "")
	              : setValueForAttribute(domElement, key, value);
	        }
	  }
	}
	function setInitialProperties(domElement, tag, props) {
	  switch (tag) {
	    case "div":
	    case "span":
	    case "svg":
	    case "path":
	    case "a":
	    case "g":
	    case "p":
	    case "li":
	      break;
	    case "img":
	      listenToNonDelegatedEvent("error", domElement);
	      listenToNonDelegatedEvent("load", domElement);
	      var hasSrc = !1,
	        hasSrcSet = !1,
	        propKey;
	      for (propKey in props)
	        if (props.hasOwnProperty(propKey)) {
	          var propValue = props[propKey];
	          if (null != propValue)
	            switch (propKey) {
	              case "src":
	                hasSrc = !0;
	                break;
	              case "srcSet":
	                hasSrcSet = !0;
	                break;
	              case "children":
	              case "dangerouslySetInnerHTML":
	                throw Error(formatProdErrorMessage(137, tag));
	              default:
	                setProp(domElement, tag, propKey, propValue, props, null);
	            }
	        }
	      hasSrcSet &&
	        setProp(domElement, tag, "srcSet", props.srcSet, props, null);
	      hasSrc && setProp(domElement, tag, "src", props.src, props, null);
	      return;
	    case "input":
	      listenToNonDelegatedEvent("invalid", domElement);
	      var defaultValue = (propKey = propValue = hasSrcSet = null),
	        checked = null,
	        defaultChecked = null;
	      for (hasSrc in props)
	        if (props.hasOwnProperty(hasSrc)) {
	          var propValue$184 = props[hasSrc];
	          if (null != propValue$184)
	            switch (hasSrc) {
	              case "name":
	                hasSrcSet = propValue$184;
	                break;
	              case "type":
	                propValue = propValue$184;
	                break;
	              case "checked":
	                checked = propValue$184;
	                break;
	              case "defaultChecked":
	                defaultChecked = propValue$184;
	                break;
	              case "value":
	                propKey = propValue$184;
	                break;
	              case "defaultValue":
	                defaultValue = propValue$184;
	                break;
	              case "children":
	              case "dangerouslySetInnerHTML":
	                if (null != propValue$184)
	                  throw Error(formatProdErrorMessage(137, tag));
	                break;
	              default:
	                setProp(domElement, tag, hasSrc, propValue$184, props, null);
	            }
	        }
	      initInput(
	        domElement,
	        propKey,
	        defaultValue,
	        checked,
	        defaultChecked,
	        propValue,
	        hasSrcSet,
	        !1
	      );
	      return;
	    case "select":
	      listenToNonDelegatedEvent("invalid", domElement);
	      hasSrc = propValue = propKey = null;
	      for (hasSrcSet in props)
	        if (
	          props.hasOwnProperty(hasSrcSet) &&
	          ((defaultValue = props[hasSrcSet]), null != defaultValue)
	        )
	          switch (hasSrcSet) {
	            case "value":
	              propKey = defaultValue;
	              break;
	            case "defaultValue":
	              propValue = defaultValue;
	              break;
	            case "multiple":
	              hasSrc = defaultValue;
	            default:
	              setProp(domElement, tag, hasSrcSet, defaultValue, props, null);
	          }
	      tag = propKey;
	      props = propValue;
	      domElement.multiple = !!hasSrc;
	      null != tag
	        ? updateOptions(domElement, !!hasSrc, tag, !1)
	        : null != props && updateOptions(domElement, !!hasSrc, props, !0);
	      return;
	    case "textarea":
	      listenToNonDelegatedEvent("invalid", domElement);
	      propKey = hasSrcSet = hasSrc = null;
	      for (propValue in props)
	        if (
	          props.hasOwnProperty(propValue) &&
	          ((defaultValue = props[propValue]), null != defaultValue)
	        )
	          switch (propValue) {
	            case "value":
	              hasSrc = defaultValue;
	              break;
	            case "defaultValue":
	              hasSrcSet = defaultValue;
	              break;
	            case "children":
	              propKey = defaultValue;
	              break;
	            case "dangerouslySetInnerHTML":
	              if (null != defaultValue) throw Error(formatProdErrorMessage(91));
	              break;
	            default:
	              setProp(domElement, tag, propValue, defaultValue, props, null);
	          }
	      initTextarea(domElement, hasSrc, hasSrcSet, propKey);
	      return;
	    case "option":
	      for (checked in props)
	        if (
	          props.hasOwnProperty(checked) &&
	          ((hasSrc = props[checked]), null != hasSrc)
	        )
	          switch (checked) {
	            case "selected":
	              domElement.selected =
	                hasSrc &&
	                "function" !== typeof hasSrc &&
	                "symbol" !== typeof hasSrc;
	              break;
	            default:
	              setProp(domElement, tag, checked, hasSrc, props, null);
	          }
	      return;
	    case "dialog":
	      listenToNonDelegatedEvent("beforetoggle", domElement);
	      listenToNonDelegatedEvent("toggle", domElement);
	      listenToNonDelegatedEvent("cancel", domElement);
	      listenToNonDelegatedEvent("close", domElement);
	      break;
	    case "iframe":
	    case "object":
	      listenToNonDelegatedEvent("load", domElement);
	      break;
	    case "video":
	    case "audio":
	      for (hasSrc = 0; hasSrc < mediaEventTypes.length; hasSrc++)
	        listenToNonDelegatedEvent(mediaEventTypes[hasSrc], domElement);
	      break;
	    case "image":
	      listenToNonDelegatedEvent("error", domElement);
	      listenToNonDelegatedEvent("load", domElement);
	      break;
	    case "details":
	      listenToNonDelegatedEvent("toggle", domElement);
	      break;
	    case "embed":
	    case "source":
	    case "link":
	      listenToNonDelegatedEvent("error", domElement),
	        listenToNonDelegatedEvent("load", domElement);
	    case "area":
	    case "base":
	    case "br":
	    case "col":
	    case "hr":
	    case "keygen":
	    case "meta":
	    case "param":
	    case "track":
	    case "wbr":
	    case "menuitem":
	      for (defaultChecked in props)
	        if (
	          props.hasOwnProperty(defaultChecked) &&
	          ((hasSrc = props[defaultChecked]), null != hasSrc)
	        )
	          switch (defaultChecked) {
	            case "children":
	            case "dangerouslySetInnerHTML":
	              throw Error(formatProdErrorMessage(137, tag));
	            default:
	              setProp(domElement, tag, defaultChecked, hasSrc, props, null);
	          }
	      return;
	    default:
	      if (isCustomElement(tag)) {
	        for (propValue$184 in props)
	          props.hasOwnProperty(propValue$184) &&
	            ((hasSrc = props[propValue$184]),
	            void 0 !== hasSrc &&
	              setPropOnCustomElement(
	                domElement,
	                tag,
	                propValue$184,
	                hasSrc,
	                props,
	                void 0
	              ));
	        return;
	      }
	  }
	  for (defaultValue in props)
	    props.hasOwnProperty(defaultValue) &&
	      ((hasSrc = props[defaultValue]),
	      null != hasSrc &&
	        setProp(domElement, tag, defaultValue, hasSrc, props, null));
	}
	function updateProperties(domElement, tag, lastProps, nextProps) {
	  switch (tag) {
	    case "div":
	    case "span":
	    case "svg":
	    case "path":
	    case "a":
	    case "g":
	    case "p":
	    case "li":
	      break;
	    case "input":
	      var name = null,
	        type = null,
	        value = null,
	        defaultValue = null,
	        lastDefaultValue = null,
	        checked = null,
	        defaultChecked = null;
	      for (propKey in lastProps) {
	        var lastProp = lastProps[propKey];
	        if (lastProps.hasOwnProperty(propKey) && null != lastProp)
	          switch (propKey) {
	            case "checked":
	              break;
	            case "value":
	              break;
	            case "defaultValue":
	              lastDefaultValue = lastProp;
	            default:
	              nextProps.hasOwnProperty(propKey) ||
	                setProp(domElement, tag, propKey, null, nextProps, lastProp);
	          }
	      }
	      for (var propKey$201 in nextProps) {
	        var propKey = nextProps[propKey$201];
	        lastProp = lastProps[propKey$201];
	        if (
	          nextProps.hasOwnProperty(propKey$201) &&
	          (null != propKey || null != lastProp)
	        )
	          switch (propKey$201) {
	            case "type":
	              type = propKey;
	              break;
	            case "name":
	              name = propKey;
	              break;
	            case "checked":
	              checked = propKey;
	              break;
	            case "defaultChecked":
	              defaultChecked = propKey;
	              break;
	            case "value":
	              value = propKey;
	              break;
	            case "defaultValue":
	              defaultValue = propKey;
	              break;
	            case "children":
	            case "dangerouslySetInnerHTML":
	              if (null != propKey)
	                throw Error(formatProdErrorMessage(137, tag));
	              break;
	            default:
	              propKey !== lastProp &&
	                setProp(
	                  domElement,
	                  tag,
	                  propKey$201,
	                  propKey,
	                  nextProps,
	                  lastProp
	                );
	          }
	      }
	      updateInput(
	        domElement,
	        value,
	        defaultValue,
	        lastDefaultValue,
	        checked,
	        defaultChecked,
	        type,
	        name
	      );
	      return;
	    case "select":
	      propKey = value = defaultValue = propKey$201 = null;
	      for (type in lastProps)
	        if (
	          ((lastDefaultValue = lastProps[type]),
	          lastProps.hasOwnProperty(type) && null != lastDefaultValue)
	        )
	          switch (type) {
	            case "value":
	              break;
	            case "multiple":
	              propKey = lastDefaultValue;
	            default:
	              nextProps.hasOwnProperty(type) ||
	                setProp(
	                  domElement,
	                  tag,
	                  type,
	                  null,
	                  nextProps,
	                  lastDefaultValue
	                );
	          }
	      for (name in nextProps)
	        if (
	          ((type = nextProps[name]),
	          (lastDefaultValue = lastProps[name]),
	          nextProps.hasOwnProperty(name) &&
	            (null != type || null != lastDefaultValue))
	        )
	          switch (name) {
	            case "value":
	              propKey$201 = type;
	              break;
	            case "defaultValue":
	              defaultValue = type;
	              break;
	            case "multiple":
	              value = type;
	            default:
	              type !== lastDefaultValue &&
	                setProp(
	                  domElement,
	                  tag,
	                  name,
	                  type,
	                  nextProps,
	                  lastDefaultValue
	                );
	          }
	      tag = defaultValue;
	      lastProps = value;
	      nextProps = propKey;
	      null != propKey$201
	        ? updateOptions(domElement, !!lastProps, propKey$201, !1)
	        : !!nextProps !== !!lastProps &&
	          (null != tag
	            ? updateOptions(domElement, !!lastProps, tag, !0)
	            : updateOptions(domElement, !!lastProps, lastProps ? [] : "", !1));
	      return;
	    case "textarea":
	      propKey = propKey$201 = null;
	      for (defaultValue in lastProps)
	        if (
	          ((name = lastProps[defaultValue]),
	          lastProps.hasOwnProperty(defaultValue) &&
	            null != name &&
	            !nextProps.hasOwnProperty(defaultValue))
	        )
	          switch (defaultValue) {
	            case "value":
	              break;
	            case "children":
	              break;
	            default:
	              setProp(domElement, tag, defaultValue, null, nextProps, name);
	          }
	      for (value in nextProps)
	        if (
	          ((name = nextProps[value]),
	          (type = lastProps[value]),
	          nextProps.hasOwnProperty(value) && (null != name || null != type))
	        )
	          switch (value) {
	            case "value":
	              propKey$201 = name;
	              break;
	            case "defaultValue":
	              propKey = name;
	              break;
	            case "children":
	              break;
	            case "dangerouslySetInnerHTML":
	              if (null != name) throw Error(formatProdErrorMessage(91));
	              break;
	            default:
	              name !== type &&
	                setProp(domElement, tag, value, name, nextProps, type);
	          }
	      updateTextarea(domElement, propKey$201, propKey);
	      return;
	    case "option":
	      for (var propKey$217 in lastProps)
	        if (
	          ((propKey$201 = lastProps[propKey$217]),
	          lastProps.hasOwnProperty(propKey$217) &&
	            null != propKey$201 &&
	            !nextProps.hasOwnProperty(propKey$217))
	        )
	          switch (propKey$217) {
	            case "selected":
	              domElement.selected = !1;
	              break;
	            default:
	              setProp(
	                domElement,
	                tag,
	                propKey$217,
	                null,
	                nextProps,
	                propKey$201
	              );
	          }
	      for (lastDefaultValue in nextProps)
	        if (
	          ((propKey$201 = nextProps[lastDefaultValue]),
	          (propKey = lastProps[lastDefaultValue]),
	          nextProps.hasOwnProperty(lastDefaultValue) &&
	            propKey$201 !== propKey &&
	            (null != propKey$201 || null != propKey))
	        )
	          switch (lastDefaultValue) {
	            case "selected":
	              domElement.selected =
	                propKey$201 &&
	                "function" !== typeof propKey$201 &&
	                "symbol" !== typeof propKey$201;
	              break;
	            default:
	              setProp(
	                domElement,
	                tag,
	                lastDefaultValue,
	                propKey$201,
	                nextProps,
	                propKey
	              );
	          }
	      return;
	    case "img":
	    case "link":
	    case "area":
	    case "base":
	    case "br":
	    case "col":
	    case "embed":
	    case "hr":
	    case "keygen":
	    case "meta":
	    case "param":
	    case "source":
	    case "track":
	    case "wbr":
	    case "menuitem":
	      for (var propKey$222 in lastProps)
	        (propKey$201 = lastProps[propKey$222]),
	          lastProps.hasOwnProperty(propKey$222) &&
	            null != propKey$201 &&
	            !nextProps.hasOwnProperty(propKey$222) &&
	            setProp(domElement, tag, propKey$222, null, nextProps, propKey$201);
	      for (checked in nextProps)
	        if (
	          ((propKey$201 = nextProps[checked]),
	          (propKey = lastProps[checked]),
	          nextProps.hasOwnProperty(checked) &&
	            propKey$201 !== propKey &&
	            (null != propKey$201 || null != propKey))
	        )
	          switch (checked) {
	            case "children":
	            case "dangerouslySetInnerHTML":
	              if (null != propKey$201)
	                throw Error(formatProdErrorMessage(137, tag));
	              break;
	            default:
	              setProp(
	                domElement,
	                tag,
	                checked,
	                propKey$201,
	                nextProps,
	                propKey
	              );
	          }
	      return;
	    default:
	      if (isCustomElement(tag)) {
	        for (var propKey$227 in lastProps)
	          (propKey$201 = lastProps[propKey$227]),
	            lastProps.hasOwnProperty(propKey$227) &&
	              void 0 !== propKey$201 &&
	              !nextProps.hasOwnProperty(propKey$227) &&
	              setPropOnCustomElement(
	                domElement,
	                tag,
	                propKey$227,
	                void 0,
	                nextProps,
	                propKey$201
	              );
	        for (defaultChecked in nextProps)
	          (propKey$201 = nextProps[defaultChecked]),
	            (propKey = lastProps[defaultChecked]),
	            !nextProps.hasOwnProperty(defaultChecked) ||
	              propKey$201 === propKey ||
	              (void 0 === propKey$201 && void 0 === propKey) ||
	              setPropOnCustomElement(
	                domElement,
	                tag,
	                defaultChecked,
	                propKey$201,
	                nextProps,
	                propKey
	              );
	        return;
	      }
	  }
	  for (var propKey$232 in lastProps)
	    (propKey$201 = lastProps[propKey$232]),
	      lastProps.hasOwnProperty(propKey$232) &&
	        null != propKey$201 &&
	        !nextProps.hasOwnProperty(propKey$232) &&
	        setProp(domElement, tag, propKey$232, null, nextProps, propKey$201);
	  for (lastProp in nextProps)
	    (propKey$201 = nextProps[lastProp]),
	      (propKey = lastProps[lastProp]),
	      !nextProps.hasOwnProperty(lastProp) ||
	        propKey$201 === propKey ||
	        (null == propKey$201 && null == propKey) ||
	        setProp(domElement, tag, lastProp, propKey$201, nextProps, propKey);
	}
	function isLikelyStaticResource(initiatorType) {
	  switch (initiatorType) {
	    case "css":
	    case "script":
	    case "font":
	    case "img":
	    case "image":
	    case "input":
	    case "link":
	      return !0;
	    default:
	      return !1;
	  }
	}
	function estimateBandwidth() {
	  if ("function" === typeof performance.getEntriesByType) {
	    for (
	      var count = 0,
	        bits = 0,
	        resourceEntries = performance.getEntriesByType("resource"),
	        i = 0;
	      i < resourceEntries.length;
	      i++
	    ) {
	      var entry = resourceEntries[i],
	        transferSize = entry.transferSize,
	        initiatorType = entry.initiatorType,
	        duration = entry.duration;
	      if (transferSize && duration && isLikelyStaticResource(initiatorType)) {
	        initiatorType = 0;
	        duration = entry.responseEnd;
	        for (i += 1; i < resourceEntries.length; i++) {
	          var overlapEntry = resourceEntries[i],
	            overlapStartTime = overlapEntry.startTime;
	          if (overlapStartTime > duration) break;
	          var overlapTransferSize = overlapEntry.transferSize,
	            overlapInitiatorType = overlapEntry.initiatorType;
	          overlapTransferSize &&
	            isLikelyStaticResource(overlapInitiatorType) &&
	            ((overlapEntry = overlapEntry.responseEnd),
	            (initiatorType +=
	              overlapTransferSize *
	              (overlapEntry < duration
	                ? 1
	                : (duration - overlapStartTime) /
	                  (overlapEntry - overlapStartTime))));
	        }
	        --i;
	        bits += (8 * (transferSize + initiatorType)) / (entry.duration / 1e3);
	        count++;
	        if (10 < count) break;
	      }
	    }
	    if (0 < count) return bits / count / 1e6;
	  }
	  return navigator.connection &&
	    ((count = navigator.connection.downlink), "number" === typeof count)
	    ? count
	    : 5;
	}
	var eventsEnabled = null,
	  selectionInformation = null;
	function getOwnerDocumentFromRootContainer(rootContainerElement) {
	  return 9 === rootContainerElement.nodeType
	    ? rootContainerElement
	    : rootContainerElement.ownerDocument;
	}
	function getOwnHostContext(namespaceURI) {
	  switch (namespaceURI) {
	    case "http://www.w3.org/2000/svg":
	      return 1;
	    case "http://www.w3.org/1998/Math/MathML":
	      return 2;
	    default:
	      return 0;
	  }
	}
	function getChildHostContextProd(parentNamespace, type) {
	  if (0 === parentNamespace)
	    switch (type) {
	      case "svg":
	        return 1;
	      case "math":
	        return 2;
	      default:
	        return 0;
	    }
	  return 1 === parentNamespace && "foreignObject" === type
	    ? 0
	    : parentNamespace;
	}
	function shouldSetTextContent(type, props) {
	  return (
	    "textarea" === type ||
	    "noscript" === type ||
	    "string" === typeof props.children ||
	    "number" === typeof props.children ||
	    "bigint" === typeof props.children ||
	    ("object" === typeof props.dangerouslySetInnerHTML &&
	      null !== props.dangerouslySetInnerHTML &&
	      null != props.dangerouslySetInnerHTML.__html)
	  );
	}
	var currentPopstateTransitionEvent = null;
	function shouldAttemptEagerTransition() {
	  var event = window.event;
	  if (event && "popstate" === event.type) {
	    if (event === currentPopstateTransitionEvent) return !1;
	    currentPopstateTransitionEvent = event;
	    return !0;
	  }
	  currentPopstateTransitionEvent = null;
	  return !1;
	}
	var scheduleTimeout = "function" === typeof setTimeout ? setTimeout : void 0,
	  cancelTimeout = "function" === typeof clearTimeout ? clearTimeout : void 0,
	  localPromise = "function" === typeof Promise ? Promise : void 0,
	  scheduleMicrotask =
	    "function" === typeof queueMicrotask
	      ? queueMicrotask
	      : "undefined" !== typeof localPromise
	        ? function (callback) {
	            return localPromise
	              .resolve(null)
	              .then(callback)
	              .catch(handleErrorInNextTick);
	          }
	        : scheduleTimeout;
	function handleErrorInNextTick(error) {
	  setTimeout(function () {
	    throw error;
	  });
	}
	function isSingletonScope(type) {
	  return "head" === type;
	}
	function clearHydrationBoundary(parentInstance, hydrationInstance) {
	  var node = hydrationInstance,
	    depth = 0;
	  do {
	    var nextNode = node.nextSibling;
	    parentInstance.removeChild(node);
	    if (nextNode && 8 === nextNode.nodeType)
	      if (((node = nextNode.data), "/$" === node || "/&" === node)) {
	        if (0 === depth) {
	          parentInstance.removeChild(nextNode);
	          retryIfBlockedOn(hydrationInstance);
	          return;
	        }
	        depth--;
	      } else if (
	        "$" === node ||
	        "$?" === node ||
	        "$~" === node ||
	        "$!" === node ||
	        "&" === node
	      )
	        depth++;
	      else if ("html" === node)
	        releaseSingletonInstance(parentInstance.ownerDocument.documentElement);
	      else if ("head" === node) {
	        node = parentInstance.ownerDocument.head;
	        releaseSingletonInstance(node);
	        for (var node$jscomp$0 = node.firstChild; node$jscomp$0; ) {
	          var nextNode$jscomp$0 = node$jscomp$0.nextSibling,
	            nodeName = node$jscomp$0.nodeName;
	          node$jscomp$0[internalHoistableMarker] ||
	            "SCRIPT" === nodeName ||
	            "STYLE" === nodeName ||
	            ("LINK" === nodeName &&
	              "stylesheet" === node$jscomp$0.rel.toLowerCase()) ||
	            node.removeChild(node$jscomp$0);
	          node$jscomp$0 = nextNode$jscomp$0;
	        }
	      } else
	        "body" === node &&
	          releaseSingletonInstance(parentInstance.ownerDocument.body);
	    node = nextNode;
	  } while (node);
	  retryIfBlockedOn(hydrationInstance);
	}
	function hideOrUnhideDehydratedBoundary(suspenseInstance, isHidden) {
	  var node = suspenseInstance;
	  suspenseInstance = 0;
	  do {
	    var nextNode = node.nextSibling;
	    1 === node.nodeType
	      ? isHidden
	        ? ((node._stashedDisplay = node.style.display),
	          (node.style.display = "none"))
	        : ((node.style.display = node._stashedDisplay || ""),
	          "" === node.getAttribute("style") && node.removeAttribute("style"))
	      : 3 === node.nodeType &&
	        (isHidden
	          ? ((node._stashedText = node.nodeValue), (node.nodeValue = ""))
	          : (node.nodeValue = node._stashedText || ""));
	    if (nextNode && 8 === nextNode.nodeType)
	      if (((node = nextNode.data), "/$" === node))
	        if (0 === suspenseInstance) break;
	        else suspenseInstance--;
	      else
	        ("$" !== node && "$?" !== node && "$~" !== node && "$!" !== node) ||
	          suspenseInstance++;
	    node = nextNode;
	  } while (node);
	}
	function clearContainerSparingly(container) {
	  var nextNode = container.firstChild;
	  nextNode && 10 === nextNode.nodeType && (nextNode = nextNode.nextSibling);
	  for (; nextNode; ) {
	    var node = nextNode;
	    nextNode = nextNode.nextSibling;
	    switch (node.nodeName) {
	      case "HTML":
	      case "HEAD":
	      case "BODY":
	        clearContainerSparingly(node);
	        detachDeletedInstance(node);
	        continue;
	      case "SCRIPT":
	      case "STYLE":
	        continue;
	      case "LINK":
	        if ("stylesheet" === node.rel.toLowerCase()) continue;
	    }
	    container.removeChild(node);
	  }
	}
	function canHydrateInstance(instance, type, props, inRootOrSingleton) {
	  for (; 1 === instance.nodeType; ) {
	    var anyProps = props;
	    if (instance.nodeName.toLowerCase() !== type.toLowerCase()) {
	      if (
	        !inRootOrSingleton &&
	        ("INPUT" !== instance.nodeName || "hidden" !== instance.type)
	      )
	        break;
	    } else if (!inRootOrSingleton)
	      if ("input" === type && "hidden" === instance.type) {
	        var name = null == anyProps.name ? null : "" + anyProps.name;
	        if (
	          "hidden" === anyProps.type &&
	          instance.getAttribute("name") === name
	        )
	          return instance;
	      } else return instance;
	    else if (!instance[internalHoistableMarker])
	      switch (type) {
	        case "meta":
	          if (!instance.hasAttribute("itemprop")) break;
	          return instance;
	        case "link":
	          name = instance.getAttribute("rel");
	          if ("stylesheet" === name && instance.hasAttribute("data-precedence"))
	            break;
	          else if (
	            name !== anyProps.rel ||
	            instance.getAttribute("href") !==
	              (null == anyProps.href || "" === anyProps.href
	                ? null
	                : anyProps.href) ||
	            instance.getAttribute("crossorigin") !==
	              (null == anyProps.crossOrigin ? null : anyProps.crossOrigin) ||
	            instance.getAttribute("title") !==
	              (null == anyProps.title ? null : anyProps.title)
	          )
	            break;
	          return instance;
	        case "style":
	          if (instance.hasAttribute("data-precedence")) break;
	          return instance;
	        case "script":
	          name = instance.getAttribute("src");
	          if (
	            (name !== (null == anyProps.src ? null : anyProps.src) ||
	              instance.getAttribute("type") !==
	                (null == anyProps.type ? null : anyProps.type) ||
	              instance.getAttribute("crossorigin") !==
	                (null == anyProps.crossOrigin ? null : anyProps.crossOrigin)) &&
	            name &&
	            instance.hasAttribute("async") &&
	            !instance.hasAttribute("itemprop")
	          )
	            break;
	          return instance;
	        default:
	          return instance;
	      }
	    instance = getNextHydratable(instance.nextSibling);
	    if (null === instance) break;
	  }
	  return null;
	}
	function canHydrateTextInstance(instance, text, inRootOrSingleton) {
	  if ("" === text) return null;
	  for (; 3 !== instance.nodeType; ) {
	    if (
	      (1 !== instance.nodeType ||
	        "INPUT" !== instance.nodeName ||
	        "hidden" !== instance.type) &&
	      !inRootOrSingleton
	    )
	      return null;
	    instance = getNextHydratable(instance.nextSibling);
	    if (null === instance) return null;
	  }
	  return instance;
	}
	function canHydrateHydrationBoundary(instance, inRootOrSingleton) {
	  for (; 8 !== instance.nodeType; ) {
	    if (
	      (1 !== instance.nodeType ||
	        "INPUT" !== instance.nodeName ||
	        "hidden" !== instance.type) &&
	      !inRootOrSingleton
	    )
	      return null;
	    instance = getNextHydratable(instance.nextSibling);
	    if (null === instance) return null;
	  }
	  return instance;
	}
	function isSuspenseInstancePending(instance) {
	  return "$?" === instance.data || "$~" === instance.data;
	}
	function isSuspenseInstanceFallback(instance) {
	  return (
	    "$!" === instance.data ||
	    ("$?" === instance.data && "loading" !== instance.ownerDocument.readyState)
	  );
	}
	function registerSuspenseInstanceRetry(instance, callback) {
	  var ownerDocument = instance.ownerDocument;
	  if ("$~" === instance.data) instance._reactRetry = callback;
	  else if ("$?" !== instance.data || "loading" !== ownerDocument.readyState)
	    callback();
	  else {
	    var listener = function () {
	      callback();
	      ownerDocument.removeEventListener("DOMContentLoaded", listener);
	    };
	    ownerDocument.addEventListener("DOMContentLoaded", listener);
	    instance._reactRetry = listener;
	  }
	}
	function getNextHydratable(node) {
	  for (; null != node; node = node.nextSibling) {
	    var nodeType = node.nodeType;
	    if (1 === nodeType || 3 === nodeType) break;
	    if (8 === nodeType) {
	      nodeType = node.data;
	      if (
	        "$" === nodeType ||
	        "$!" === nodeType ||
	        "$?" === nodeType ||
	        "$~" === nodeType ||
	        "&" === nodeType ||
	        "F!" === nodeType ||
	        "F" === nodeType
	      )
	        break;
	      if ("/$" === nodeType || "/&" === nodeType) return null;
	    }
	  }
	  return node;
	}
	var previousHydratableOnEnteringScopedSingleton = null;
	function getNextHydratableInstanceAfterHydrationBoundary(hydrationInstance) {
	  hydrationInstance = hydrationInstance.nextSibling;
	  for (var depth = 0; hydrationInstance; ) {
	    if (8 === hydrationInstance.nodeType) {
	      var data = hydrationInstance.data;
	      if ("/$" === data || "/&" === data) {
	        if (0 === depth)
	          return getNextHydratable(hydrationInstance.nextSibling);
	        depth--;
	      } else
	        ("$" !== data &&
	          "$!" !== data &&
	          "$?" !== data &&
	          "$~" !== data &&
	          "&" !== data) ||
	          depth++;
	    }
	    hydrationInstance = hydrationInstance.nextSibling;
	  }
	  return null;
	}
	function getParentHydrationBoundary(targetInstance) {
	  targetInstance = targetInstance.previousSibling;
	  for (var depth = 0; targetInstance; ) {
	    if (8 === targetInstance.nodeType) {
	      var data = targetInstance.data;
	      if (
	        "$" === data ||
	        "$!" === data ||
	        "$?" === data ||
	        "$~" === data ||
	        "&" === data
	      ) {
	        if (0 === depth) return targetInstance;
	        depth--;
	      } else ("/$" !== data && "/&" !== data) || depth++;
	    }
	    targetInstance = targetInstance.previousSibling;
	  }
	  return null;
	}
	function resolveSingletonInstance(type, props, rootContainerInstance) {
	  props = getOwnerDocumentFromRootContainer(rootContainerInstance);
	  switch (type) {
	    case "html":
	      type = props.documentElement;
	      if (!type) throw Error(formatProdErrorMessage(452));
	      return type;
	    case "head":
	      type = props.head;
	      if (!type) throw Error(formatProdErrorMessage(453));
	      return type;
	    case "body":
	      type = props.body;
	      if (!type) throw Error(formatProdErrorMessage(454));
	      return type;
	    default:
	      throw Error(formatProdErrorMessage(451));
	  }
	}
	function releaseSingletonInstance(instance) {
	  for (var attributes = instance.attributes; attributes.length; )
	    instance.removeAttributeNode(attributes[0]);
	  detachDeletedInstance(instance);
	}
	var preloadPropsMap = new Map(),
	  preconnectsSet = new Set();
	function getHoistableRoot(container) {
	  return "function" === typeof container.getRootNode
	    ? container.getRootNode()
	    : 9 === container.nodeType
	      ? container
	      : container.ownerDocument;
	}
	var previousDispatcher = ReactDOMSharedInternals.d;
	ReactDOMSharedInternals.d = {
	  f: flushSyncWork,
	  r: requestFormReset,
	  D: prefetchDNS,
	  C: preconnect,
	  L: preload,
	  m: preloadModule,
	  X: preinitScript,
	  S: preinitStyle,
	  M: preinitModuleScript
	};
	function flushSyncWork() {
	  var previousWasRendering = previousDispatcher.f(),
	    wasRendering = flushSyncWork$1();
	  return previousWasRendering || wasRendering;
	}
	function requestFormReset(form) {
	  var formInst = getInstanceFromNode(form);
	  null !== formInst && 5 === formInst.tag && "form" === formInst.type
	    ? requestFormReset$1(formInst)
	    : previousDispatcher.r(form);
	}
	var globalDocument = "undefined" === typeof document ? null : document;
	function preconnectAs(rel, href, crossOrigin) {
	  var ownerDocument = globalDocument;
	  if (ownerDocument && "string" === typeof href && href) {
	    var limitedEscapedHref =
	      escapeSelectorAttributeValueInsideDoubleQuotes(href);
	    limitedEscapedHref =
	      'link[rel="' + rel + '"][href="' + limitedEscapedHref + '"]';
	    "string" === typeof crossOrigin &&
	      (limitedEscapedHref += '[crossorigin="' + crossOrigin + '"]');
	    preconnectsSet.has(limitedEscapedHref) ||
	      (preconnectsSet.add(limitedEscapedHref),
	      (rel = { rel: rel, crossOrigin: crossOrigin, href: href }),
	      null === ownerDocument.querySelector(limitedEscapedHref) &&
	        ((href = ownerDocument.createElement("link")),
	        setInitialProperties(href, "link", rel),
	        markNodeAsHoistable(href),
	        ownerDocument.head.appendChild(href)));
	  }
	}
	function prefetchDNS(href) {
	  previousDispatcher.D(href);
	  preconnectAs("dns-prefetch", href, null);
	}
	function preconnect(href, crossOrigin) {
	  previousDispatcher.C(href, crossOrigin);
	  preconnectAs("preconnect", href, crossOrigin);
	}
	function preload(href, as, options) {
	  previousDispatcher.L(href, as, options);
	  var ownerDocument = globalDocument;
	  if (ownerDocument && href && as) {
	    var preloadSelector =
	      'link[rel="preload"][as="' +
	      escapeSelectorAttributeValueInsideDoubleQuotes(as) +
	      '"]';
	    "image" === as
	      ? options && options.imageSrcSet
	        ? ((preloadSelector +=
	            '[imagesrcset="' +
	            escapeSelectorAttributeValueInsideDoubleQuotes(
	              options.imageSrcSet
	            ) +
	            '"]'),
	          "string" === typeof options.imageSizes &&
	            (preloadSelector +=
	              '[imagesizes="' +
	              escapeSelectorAttributeValueInsideDoubleQuotes(
	                options.imageSizes
	              ) +
	              '"]'))
	        : (preloadSelector +=
	            '[href="' +
	            escapeSelectorAttributeValueInsideDoubleQuotes(href) +
	            '"]')
	      : (preloadSelector +=
	          '[href="' +
	          escapeSelectorAttributeValueInsideDoubleQuotes(href) +
	          '"]');
	    var key = preloadSelector;
	    switch (as) {
	      case "style":
	        key = getStyleKey(href);
	        break;
	      case "script":
	        key = getScriptKey(href);
	    }
	    preloadPropsMap.has(key) ||
	      ((href = assign(
	        {
	          rel: "preload",
	          href:
	            "image" === as && options && options.imageSrcSet ? void 0 : href,
	          as: as
	        },
	        options
	      )),
	      preloadPropsMap.set(key, href),
	      null !== ownerDocument.querySelector(preloadSelector) ||
	        ("style" === as &&
	          ownerDocument.querySelector(getStylesheetSelectorFromKey(key))) ||
	        ("script" === as &&
	          ownerDocument.querySelector(getScriptSelectorFromKey(key))) ||
	        ((as = ownerDocument.createElement("link")),
	        setInitialProperties(as, "link", href),
	        markNodeAsHoistable(as),
	        ownerDocument.head.appendChild(as)));
	  }
	}
	function preloadModule(href, options) {
	  previousDispatcher.m(href, options);
	  var ownerDocument = globalDocument;
	  if (ownerDocument && href) {
	    var as = options && "string" === typeof options.as ? options.as : "script",
	      preloadSelector =
	        'link[rel="modulepreload"][as="' +
	        escapeSelectorAttributeValueInsideDoubleQuotes(as) +
	        '"][href="' +
	        escapeSelectorAttributeValueInsideDoubleQuotes(href) +
	        '"]',
	      key = preloadSelector;
	    switch (as) {
	      case "audioworklet":
	      case "paintworklet":
	      case "serviceworker":
	      case "sharedworker":
	      case "worker":
	      case "script":
	        key = getScriptKey(href);
	    }
	    if (
	      !preloadPropsMap.has(key) &&
	      ((href = assign({ rel: "modulepreload", href: href }, options)),
	      preloadPropsMap.set(key, href),
	      null === ownerDocument.querySelector(preloadSelector))
	    ) {
	      switch (as) {
	        case "audioworklet":
	        case "paintworklet":
	        case "serviceworker":
	        case "sharedworker":
	        case "worker":
	        case "script":
	          if (ownerDocument.querySelector(getScriptSelectorFromKey(key)))
	            return;
	      }
	      as = ownerDocument.createElement("link");
	      setInitialProperties(as, "link", href);
	      markNodeAsHoistable(as);
	      ownerDocument.head.appendChild(as);
	    }
	  }
	}
	function preinitStyle(href, precedence, options) {
	  previousDispatcher.S(href, precedence, options);
	  var ownerDocument = globalDocument;
	  if (ownerDocument && href) {
	    var styles = getResourcesFromRoot(ownerDocument).hoistableStyles,
	      key = getStyleKey(href);
	    precedence = precedence || "default";
	    var resource = styles.get(key);
	    if (!resource) {
	      var state = { loading: 0, preload: null };
	      if (
	        (resource = ownerDocument.querySelector(
	          getStylesheetSelectorFromKey(key)
	        ))
	      )
	        state.loading = 5;
	      else {
	        href = assign(
	          { rel: "stylesheet", href: href, "data-precedence": precedence },
	          options
	        );
	        (options = preloadPropsMap.get(key)) &&
	          adoptPreloadPropsForStylesheet(href, options);
	        var link = (resource = ownerDocument.createElement("link"));
	        markNodeAsHoistable(link);
	        setInitialProperties(link, "link", href);
	        link._p = new Promise(function (resolve, reject) {
	          link.onload = resolve;
	          link.onerror = reject;
	        });
	        link.addEventListener("load", function () {
	          state.loading |= 1;
	        });
	        link.addEventListener("error", function () {
	          state.loading |= 2;
	        });
	        state.loading |= 4;
	        insertStylesheet(resource, precedence, ownerDocument);
	      }
	      resource = {
	        type: "stylesheet",
	        instance: resource,
	        count: 1,
	        state: state
	      };
	      styles.set(key, resource);
	    }
	  }
	}
	function preinitScript(src, options) {
	  previousDispatcher.X(src, options);
	  var ownerDocument = globalDocument;
	  if (ownerDocument && src) {
	    var scripts = getResourcesFromRoot(ownerDocument).hoistableScripts,
	      key = getScriptKey(src),
	      resource = scripts.get(key);
	    resource ||
	      ((resource = ownerDocument.querySelector(getScriptSelectorFromKey(key))),
	      resource ||
	        ((src = assign({ src: src, async: !0 }, options)),
	        (options = preloadPropsMap.get(key)) &&
	          adoptPreloadPropsForScript(src, options),
	        (resource = ownerDocument.createElement("script")),
	        markNodeAsHoistable(resource),
	        setInitialProperties(resource, "link", src),
	        ownerDocument.head.appendChild(resource)),
	      (resource = {
	        type: "script",
	        instance: resource,
	        count: 1,
	        state: null
	      }),
	      scripts.set(key, resource));
	  }
	}
	function preinitModuleScript(src, options) {
	  previousDispatcher.M(src, options);
	  var ownerDocument = globalDocument;
	  if (ownerDocument && src) {
	    var scripts = getResourcesFromRoot(ownerDocument).hoistableScripts,
	      key = getScriptKey(src),
	      resource = scripts.get(key);
	    resource ||
	      ((resource = ownerDocument.querySelector(getScriptSelectorFromKey(key))),
	      resource ||
	        ((src = assign({ src: src, async: !0, type: "module" }, options)),
	        (options = preloadPropsMap.get(key)) &&
	          adoptPreloadPropsForScript(src, options),
	        (resource = ownerDocument.createElement("script")),
	        markNodeAsHoistable(resource),
	        setInitialProperties(resource, "link", src),
	        ownerDocument.head.appendChild(resource)),
	      (resource = {
	        type: "script",
	        instance: resource,
	        count: 1,
	        state: null
	      }),
	      scripts.set(key, resource));
	  }
	}
	function getResource(type, currentProps, pendingProps, currentResource) {
	  var JSCompiler_inline_result = (JSCompiler_inline_result =
	    rootInstanceStackCursor.current)
	    ? getHoistableRoot(JSCompiler_inline_result)
	    : null;
	  if (!JSCompiler_inline_result) throw Error(formatProdErrorMessage(446));
	  switch (type) {
	    case "meta":
	    case "title":
	      return null;
	    case "style":
	      return "string" === typeof pendingProps.precedence &&
	        "string" === typeof pendingProps.href
	        ? ((currentProps = getStyleKey(pendingProps.href)),
	          (pendingProps = getResourcesFromRoot(
	            JSCompiler_inline_result
	          ).hoistableStyles),
	          (currentResource = pendingProps.get(currentProps)),
	          currentResource ||
	            ((currentResource = {
	              type: "style",
	              instance: null,
	              count: 0,
	              state: null
	            }),
	            pendingProps.set(currentProps, currentResource)),
	          currentResource)
	        : { type: "void", instance: null, count: 0, state: null };
	    case "link":
	      if (
	        "stylesheet" === pendingProps.rel &&
	        "string" === typeof pendingProps.href &&
	        "string" === typeof pendingProps.precedence
	      ) {
	        type = getStyleKey(pendingProps.href);
	        var styles$243 = getResourcesFromRoot(
	            JSCompiler_inline_result
	          ).hoistableStyles,
	          resource$244 = styles$243.get(type);
	        resource$244 ||
	          ((JSCompiler_inline_result =
	            JSCompiler_inline_result.ownerDocument || JSCompiler_inline_result),
	          (resource$244 = {
	            type: "stylesheet",
	            instance: null,
	            count: 0,
	            state: { loading: 0, preload: null }
	          }),
	          styles$243.set(type, resource$244),
	          (styles$243 = JSCompiler_inline_result.querySelector(
	            getStylesheetSelectorFromKey(type)
	          )) &&
	            !styles$243._p &&
	            ((resource$244.instance = styles$243),
	            (resource$244.state.loading = 5)),
	          preloadPropsMap.has(type) ||
	            ((pendingProps = {
	              rel: "preload",
	              as: "style",
	              href: pendingProps.href,
	              crossOrigin: pendingProps.crossOrigin,
	              integrity: pendingProps.integrity,
	              media: pendingProps.media,
	              hrefLang: pendingProps.hrefLang,
	              referrerPolicy: pendingProps.referrerPolicy
	            }),
	            preloadPropsMap.set(type, pendingProps),
	            styles$243 ||
	              preloadStylesheet(
	                JSCompiler_inline_result,
	                type,
	                pendingProps,
	                resource$244.state
	              )));
	        if (currentProps && null === currentResource)
	          throw Error(formatProdErrorMessage(528, ""));
	        return resource$244;
	      }
	      if (currentProps && null !== currentResource)
	        throw Error(formatProdErrorMessage(529, ""));
	      return null;
	    case "script":
	      return (
	        (currentProps = pendingProps.async),
	        (pendingProps = pendingProps.src),
	        "string" === typeof pendingProps &&
	        currentProps &&
	        "function" !== typeof currentProps &&
	        "symbol" !== typeof currentProps
	          ? ((currentProps = getScriptKey(pendingProps)),
	            (pendingProps = getResourcesFromRoot(
	              JSCompiler_inline_result
	            ).hoistableScripts),
	            (currentResource = pendingProps.get(currentProps)),
	            currentResource ||
	              ((currentResource = {
	                type: "script",
	                instance: null,
	                count: 0,
	                state: null
	              }),
	              pendingProps.set(currentProps, currentResource)),
	            currentResource)
	          : { type: "void", instance: null, count: 0, state: null }
	      );
	    default:
	      throw Error(formatProdErrorMessage(444, type));
	  }
	}
	function getStyleKey(href) {
	  return 'href="' + escapeSelectorAttributeValueInsideDoubleQuotes(href) + '"';
	}
	function getStylesheetSelectorFromKey(key) {
	  return 'link[rel="stylesheet"][' + key + "]";
	}
	function stylesheetPropsFromRawProps(rawProps) {
	  return assign({}, rawProps, {
	    "data-precedence": rawProps.precedence,
	    precedence: null
	  });
	}
	function preloadStylesheet(ownerDocument, key, preloadProps, state) {
	  ownerDocument.querySelector('link[rel="preload"][as="style"][' + key + "]")
	    ? (state.loading = 1)
	    : ((key = ownerDocument.createElement("link")),
	      (state.preload = key),
	      key.addEventListener("load", function () {
	        return (state.loading |= 1);
	      }),
	      key.addEventListener("error", function () {
	        return (state.loading |= 2);
	      }),
	      setInitialProperties(key, "link", preloadProps),
	      markNodeAsHoistable(key),
	      ownerDocument.head.appendChild(key));
	}
	function getScriptKey(src) {
	  return '[src="' + escapeSelectorAttributeValueInsideDoubleQuotes(src) + '"]';
	}
	function getScriptSelectorFromKey(key) {
	  return "script[async]" + key;
	}
	function acquireResource(hoistableRoot, resource, props) {
	  resource.count++;
	  if (null === resource.instance)
	    switch (resource.type) {
	      case "style":
	        var instance = hoistableRoot.querySelector(
	          'style[data-href~="' +
	            escapeSelectorAttributeValueInsideDoubleQuotes(props.href) +
	            '"]'
	        );
	        if (instance)
	          return (
	            (resource.instance = instance),
	            markNodeAsHoistable(instance),
	            instance
	          );
	        var styleProps = assign({}, props, {
	          "data-href": props.href,
	          "data-precedence": props.precedence,
	          href: null,
	          precedence: null
	        });
	        instance = (hoistableRoot.ownerDocument || hoistableRoot).createElement(
	          "style"
	        );
	        markNodeAsHoistable(instance);
	        setInitialProperties(instance, "style", styleProps);
	        insertStylesheet(instance, props.precedence, hoistableRoot);
	        return (resource.instance = instance);
	      case "stylesheet":
	        styleProps = getStyleKey(props.href);
	        var instance$249 = hoistableRoot.querySelector(
	          getStylesheetSelectorFromKey(styleProps)
	        );
	        if (instance$249)
	          return (
	            (resource.state.loading |= 4),
	            (resource.instance = instance$249),
	            markNodeAsHoistable(instance$249),
	            instance$249
	          );
	        instance = stylesheetPropsFromRawProps(props);
	        (styleProps = preloadPropsMap.get(styleProps)) &&
	          adoptPreloadPropsForStylesheet(instance, styleProps);
	        instance$249 = (
	          hoistableRoot.ownerDocument || hoistableRoot
	        ).createElement("link");
	        markNodeAsHoistable(instance$249);
	        var linkInstance = instance$249;
	        linkInstance._p = new Promise(function (resolve, reject) {
	          linkInstance.onload = resolve;
	          linkInstance.onerror = reject;
	        });
	        setInitialProperties(instance$249, "link", instance);
	        resource.state.loading |= 4;
	        insertStylesheet(instance$249, props.precedence, hoistableRoot);
	        return (resource.instance = instance$249);
	      case "script":
	        instance$249 = getScriptKey(props.src);
	        if (
	          (styleProps = hoistableRoot.querySelector(
	            getScriptSelectorFromKey(instance$249)
	          ))
	        )
	          return (
	            (resource.instance = styleProps),
	            markNodeAsHoistable(styleProps),
	            styleProps
	          );
	        instance = props;
	        if ((styleProps = preloadPropsMap.get(instance$249)))
	          (instance = assign({}, props)),
	            adoptPreloadPropsForScript(instance, styleProps);
	        hoistableRoot = hoistableRoot.ownerDocument || hoistableRoot;
	        styleProps = hoistableRoot.createElement("script");
	        markNodeAsHoistable(styleProps);
	        setInitialProperties(styleProps, "link", instance);
	        hoistableRoot.head.appendChild(styleProps);
	        return (resource.instance = styleProps);
	      case "void":
	        return null;
	      default:
	        throw Error(formatProdErrorMessage(443, resource.type));
	    }
	  else
	    "stylesheet" === resource.type &&
	      0 === (resource.state.loading & 4) &&
	      ((instance = resource.instance),
	      (resource.state.loading |= 4),
	      insertStylesheet(instance, props.precedence, hoistableRoot));
	  return resource.instance;
	}
	function insertStylesheet(instance, precedence, root) {
	  for (
	    var nodes = root.querySelectorAll(
	        'link[rel="stylesheet"][data-precedence],style[data-precedence]'
	      ),
	      last = nodes.length ? nodes[nodes.length - 1] : null,
	      prior = last,
	      i = 0;
	    i < nodes.length;
	    i++
	  ) {
	    var node = nodes[i];
	    if (node.dataset.precedence === precedence) prior = node;
	    else if (prior !== last) break;
	  }
	  prior
	    ? prior.parentNode.insertBefore(instance, prior.nextSibling)
	    : ((precedence = 9 === root.nodeType ? root.head : root),
	      precedence.insertBefore(instance, precedence.firstChild));
	}
	function adoptPreloadPropsForStylesheet(stylesheetProps, preloadProps) {
	  null == stylesheetProps.crossOrigin &&
	    (stylesheetProps.crossOrigin = preloadProps.crossOrigin);
	  null == stylesheetProps.referrerPolicy &&
	    (stylesheetProps.referrerPolicy = preloadProps.referrerPolicy);
	  null == stylesheetProps.title && (stylesheetProps.title = preloadProps.title);
	}
	function adoptPreloadPropsForScript(scriptProps, preloadProps) {
	  null == scriptProps.crossOrigin &&
	    (scriptProps.crossOrigin = preloadProps.crossOrigin);
	  null == scriptProps.referrerPolicy &&
	    (scriptProps.referrerPolicy = preloadProps.referrerPolicy);
	  null == scriptProps.integrity &&
	    (scriptProps.integrity = preloadProps.integrity);
	}
	var tagCaches = null;
	function getHydratableHoistableCache(type, keyAttribute, ownerDocument) {
	  if (null === tagCaches) {
	    var cache = new Map();
	    var caches = (tagCaches = new Map());
	    caches.set(ownerDocument, cache);
	  } else
	    (caches = tagCaches),
	      (cache = caches.get(ownerDocument)),
	      cache || ((cache = new Map()), caches.set(ownerDocument, cache));
	  if (cache.has(type)) return cache;
	  cache.set(type, null);
	  ownerDocument = ownerDocument.getElementsByTagName(type);
	  for (caches = 0; caches < ownerDocument.length; caches++) {
	    var node = ownerDocument[caches];
	    if (
	      !(
	        node[internalHoistableMarker] ||
	        node[internalInstanceKey] ||
	        ("link" === type && "stylesheet" === node.getAttribute("rel"))
	      ) &&
	      "http://www.w3.org/2000/svg" !== node.namespaceURI
	    ) {
	      var nodeKey = node.getAttribute(keyAttribute) || "";
	      nodeKey = type + nodeKey;
	      var existing = cache.get(nodeKey);
	      existing ? existing.push(node) : cache.set(nodeKey, [node]);
	    }
	  }
	  return cache;
	}
	function mountHoistable(hoistableRoot, type, instance) {
	  hoistableRoot = hoistableRoot.ownerDocument || hoistableRoot;
	  hoistableRoot.head.insertBefore(
	    instance,
	    "title" === type ? hoistableRoot.querySelector("head > title") : null
	  );
	}
	function isHostHoistableType(type, props, hostContext) {
	  if (1 === hostContext || null != props.itemProp) return !1;
	  switch (type) {
	    case "meta":
	    case "title":
	      return !0;
	    case "style":
	      if (
	        "string" !== typeof props.precedence ||
	        "string" !== typeof props.href ||
	        "" === props.href
	      )
	        break;
	      return !0;
	    case "link":
	      if (
	        "string" !== typeof props.rel ||
	        "string" !== typeof props.href ||
	        "" === props.href ||
	        props.onLoad ||
	        props.onError
	      )
	        break;
	      switch (props.rel) {
	        case "stylesheet":
	          return (
	            (type = props.disabled),
	            "string" === typeof props.precedence && null == type
	          );
	        default:
	          return !0;
	      }
	    case "script":
	      if (
	        props.async &&
	        "function" !== typeof props.async &&
	        "symbol" !== typeof props.async &&
	        !props.onLoad &&
	        !props.onError &&
	        props.src &&
	        "string" === typeof props.src
	      )
	        return !0;
	  }
	  return !1;
	}
	function preloadResource(resource) {
	  return "stylesheet" === resource.type && 0 === (resource.state.loading & 3)
	    ? !1
	    : !0;
	}
	function suspendResource(state, hoistableRoot, resource, props) {
	  if (
	    "stylesheet" === resource.type &&
	    ("string" !== typeof props.media ||
	      !1 !== matchMedia(props.media).matches) &&
	    0 === (resource.state.loading & 4)
	  ) {
	    if (null === resource.instance) {
	      var key = getStyleKey(props.href),
	        instance = hoistableRoot.querySelector(
	          getStylesheetSelectorFromKey(key)
	        );
	      if (instance) {
	        hoistableRoot = instance._p;
	        null !== hoistableRoot &&
	          "object" === typeof hoistableRoot &&
	          "function" === typeof hoistableRoot.then &&
	          (state.count++,
	          (state = onUnsuspend.bind(state)),
	          hoistableRoot.then(state, state));
	        resource.state.loading |= 4;
	        resource.instance = instance;
	        markNodeAsHoistable(instance);
	        return;
	      }
	      instance = hoistableRoot.ownerDocument || hoistableRoot;
	      props = stylesheetPropsFromRawProps(props);
	      (key = preloadPropsMap.get(key)) &&
	        adoptPreloadPropsForStylesheet(props, key);
	      instance = instance.createElement("link");
	      markNodeAsHoistable(instance);
	      var linkInstance = instance;
	      linkInstance._p = new Promise(function (resolve, reject) {
	        linkInstance.onload = resolve;
	        linkInstance.onerror = reject;
	      });
	      setInitialProperties(instance, "link", props);
	      resource.instance = instance;
	    }
	    null === state.stylesheets && (state.stylesheets = new Map());
	    state.stylesheets.set(resource, hoistableRoot);
	    (hoistableRoot = resource.state.preload) &&
	      0 === (resource.state.loading & 3) &&
	      (state.count++,
	      (resource = onUnsuspend.bind(state)),
	      hoistableRoot.addEventListener("load", resource),
	      hoistableRoot.addEventListener("error", resource));
	  }
	}
	var estimatedBytesWithinLimit = 0;
	function waitForCommitToBeReady(state, timeoutOffset) {
	  state.stylesheets &&
	    0 === state.count &&
	    insertSuspendedStylesheets(state, state.stylesheets);
	  return 0 < state.count || 0 < state.imgCount
	    ? function (commit) {
	        var stylesheetTimer = setTimeout(function () {
	          state.stylesheets &&
	            insertSuspendedStylesheets(state, state.stylesheets);
	          if (state.unsuspend) {
	            var unsuspend = state.unsuspend;
	            state.unsuspend = null;
	            unsuspend();
	          }
	        }, 6e4 + timeoutOffset);
	        0 < state.imgBytes &&
	          0 === estimatedBytesWithinLimit &&
	          (estimatedBytesWithinLimit = 62500 * estimateBandwidth());
	        var imgTimer = setTimeout(
	          function () {
	            state.waitingForImages = !1;
	            if (
	              0 === state.count &&
	              (state.stylesheets &&
	                insertSuspendedStylesheets(state, state.stylesheets),
	              state.unsuspend)
	            ) {
	              var unsuspend = state.unsuspend;
	              state.unsuspend = null;
	              unsuspend();
	            }
	          },
	          (state.imgBytes > estimatedBytesWithinLimit ? 50 : 800) +
	            timeoutOffset
	        );
	        state.unsuspend = commit;
	        return function () {
	          state.unsuspend = null;
	          clearTimeout(stylesheetTimer);
	          clearTimeout(imgTimer);
	        };
	      }
	    : null;
	}
	function onUnsuspend() {
	  this.count--;
	  if (0 === this.count && (0 === this.imgCount || !this.waitingForImages))
	    if (this.stylesheets) insertSuspendedStylesheets(this, this.stylesheets);
	    else if (this.unsuspend) {
	      var unsuspend = this.unsuspend;
	      this.unsuspend = null;
	      unsuspend();
	    }
	}
	var precedencesByRoot = null;
	function insertSuspendedStylesheets(state, resources) {
	  state.stylesheets = null;
	  null !== state.unsuspend &&
	    (state.count++,
	    (precedencesByRoot = new Map()),
	    resources.forEach(insertStylesheetIntoRoot, state),
	    (precedencesByRoot = null),
	    onUnsuspend.call(state));
	}
	function insertStylesheetIntoRoot(root, resource) {
	  if (!(resource.state.loading & 4)) {
	    var precedences = precedencesByRoot.get(root);
	    if (precedences) var last = precedences.get(null);
	    else {
	      precedences = new Map();
	      precedencesByRoot.set(root, precedences);
	      for (
	        var nodes = root.querySelectorAll(
	            "link[data-precedence],style[data-precedence]"
	          ),
	          i = 0;
	        i < nodes.length;
	        i++
	      ) {
	        var node = nodes[i];
	        if (
	          "LINK" === node.nodeName ||
	          "not all" !== node.getAttribute("media")
	        )
	          precedences.set(node.dataset.precedence, node), (last = node);
	      }
	      last && precedences.set(null, last);
	    }
	    nodes = resource.instance;
	    node = nodes.getAttribute("data-precedence");
	    i = precedences.get(node) || last;
	    i === last && precedences.set(null, nodes);
	    precedences.set(node, nodes);
	    this.count++;
	    last = onUnsuspend.bind(this);
	    nodes.addEventListener("load", last);
	    nodes.addEventListener("error", last);
	    i
	      ? i.parentNode.insertBefore(nodes, i.nextSibling)
	      : ((root = 9 === root.nodeType ? root.head : root),
	        root.insertBefore(nodes, root.firstChild));
	    resource.state.loading |= 4;
	  }
	}
	var HostTransitionContext = {
	  $$typeof: REACT_CONTEXT_TYPE,
	  Provider: null,
	  Consumer: null,
	  _currentValue: sharedNotPendingObject,
	  _currentValue2: sharedNotPendingObject,
	  _threadCount: 0
	};
	function FiberRootNode(
	  containerInfo,
	  tag,
	  hydrate,
	  identifierPrefix,
	  onUncaughtError,
	  onCaughtError,
	  onRecoverableError,
	  onDefaultTransitionIndicator,
	  formState
	) {
	  this.tag = 1;
	  this.containerInfo = containerInfo;
	  this.pingCache = this.current = this.pendingChildren = null;
	  this.timeoutHandle = -1;
	  this.callbackNode =
	    this.next =
	    this.pendingContext =
	    this.context =
	    this.cancelPendingCommit =
	      null;
	  this.callbackPriority = 0;
	  this.expirationTimes = createLaneMap(-1);
	  this.entangledLanes =
	    this.shellSuspendCounter =
	    this.errorRecoveryDisabledLanes =
	    this.expiredLanes =
	    this.warmLanes =
	    this.pingedLanes =
	    this.suspendedLanes =
	    this.pendingLanes =
	      0;
	  this.entanglements = createLaneMap(0);
	  this.hiddenUpdates = createLaneMap(null);
	  this.identifierPrefix = identifierPrefix;
	  this.onUncaughtError = onUncaughtError;
	  this.onCaughtError = onCaughtError;
	  this.onRecoverableError = onRecoverableError;
	  this.pooledCache = null;
	  this.pooledCacheLanes = 0;
	  this.formState = formState;
	  this.incompleteTransitions = new Map();
	}
	function createFiberRoot(
	  containerInfo,
	  tag,
	  hydrate,
	  initialChildren,
	  hydrationCallbacks,
	  isStrictMode,
	  identifierPrefix,
	  formState,
	  onUncaughtError,
	  onCaughtError,
	  onRecoverableError,
	  onDefaultTransitionIndicator
	) {
	  containerInfo = new FiberRootNode(
	    containerInfo,
	    tag,
	    hydrate,
	    identifierPrefix,
	    onUncaughtError,
	    onCaughtError,
	    onRecoverableError,
	    onDefaultTransitionIndicator,
	    formState
	  );
	  tag = 1;
	  !0 === isStrictMode && (tag |= 24);
	  isStrictMode = createFiberImplClass(3, null, null, tag);
	  containerInfo.current = isStrictMode;
	  isStrictMode.stateNode = containerInfo;
	  tag = createCache();
	  tag.refCount++;
	  containerInfo.pooledCache = tag;
	  tag.refCount++;
	  isStrictMode.memoizedState = {
	    element: initialChildren,
	    isDehydrated: hydrate,
	    cache: tag
	  };
	  initializeUpdateQueue(isStrictMode);
	  return containerInfo;
	}
	function getContextForSubtree(parentComponent) {
	  if (!parentComponent) return emptyContextObject;
	  parentComponent = emptyContextObject;
	  return parentComponent;
	}
	function updateContainerImpl(
	  rootFiber,
	  lane,
	  element,
	  container,
	  parentComponent,
	  callback
	) {
	  parentComponent = getContextForSubtree(parentComponent);
	  null === container.context
	    ? (container.context = parentComponent)
	    : (container.pendingContext = parentComponent);
	  container = createUpdate(lane);
	  container.payload = { element: element };
	  callback = void 0 === callback ? null : callback;
	  null !== callback && (container.callback = callback);
	  element = enqueueUpdate(rootFiber, container, lane);
	  null !== element &&
	    (scheduleUpdateOnFiber(element, rootFiber, lane),
	    entangleTransitions(element, rootFiber, lane));
	}
	function markRetryLaneImpl(fiber, retryLane) {
	  fiber = fiber.memoizedState;
	  if (null !== fiber && null !== fiber.dehydrated) {
	    var a = fiber.retryLane;
	    fiber.retryLane = 0 !== a && a < retryLane ? a : retryLane;
	  }
	}
	function markRetryLaneIfNotHydrated(fiber, retryLane) {
	  markRetryLaneImpl(fiber, retryLane);
	  (fiber = fiber.alternate) && markRetryLaneImpl(fiber, retryLane);
	}
	function attemptContinuousHydration(fiber) {
	  if (13 === fiber.tag || 31 === fiber.tag) {
	    var root = enqueueConcurrentRenderForLane(fiber, 67108864);
	    null !== root && scheduleUpdateOnFiber(root, fiber, 67108864);
	    markRetryLaneIfNotHydrated(fiber, 67108864);
	  }
	}
	function attemptHydrationAtCurrentPriority(fiber) {
	  if (13 === fiber.tag || 31 === fiber.tag) {
	    var lane = requestUpdateLane();
	    lane = getBumpedLaneForHydrationByLane(lane);
	    var root = enqueueConcurrentRenderForLane(fiber, lane);
	    null !== root && scheduleUpdateOnFiber(root, fiber, lane);
	    markRetryLaneIfNotHydrated(fiber, lane);
	  }
	}
	var _enabled = !0;
	function dispatchDiscreteEvent(
	  domEventName,
	  eventSystemFlags,
	  container,
	  nativeEvent
	) {
	  var prevTransition = ReactSharedInternals.T;
	  ReactSharedInternals.T = null;
	  var previousPriority = ReactDOMSharedInternals.p;
	  try {
	    (ReactDOMSharedInternals.p = 2),
	      dispatchEvent(domEventName, eventSystemFlags, container, nativeEvent);
	  } finally {
	    (ReactDOMSharedInternals.p = previousPriority),
	      (ReactSharedInternals.T = prevTransition);
	  }
	}
	function dispatchContinuousEvent(
	  domEventName,
	  eventSystemFlags,
	  container,
	  nativeEvent
	) {
	  var prevTransition = ReactSharedInternals.T;
	  ReactSharedInternals.T = null;
	  var previousPriority = ReactDOMSharedInternals.p;
	  try {
	    (ReactDOMSharedInternals.p = 8),
	      dispatchEvent(domEventName, eventSystemFlags, container, nativeEvent);
	  } finally {
	    (ReactDOMSharedInternals.p = previousPriority),
	      (ReactSharedInternals.T = prevTransition);
	  }
	}
	function dispatchEvent(
	  domEventName,
	  eventSystemFlags,
	  targetContainer,
	  nativeEvent
	) {
	  if (_enabled) {
	    var blockedOn = findInstanceBlockingEvent(nativeEvent);
	    if (null === blockedOn)
	      dispatchEventForPluginEventSystem(
	        domEventName,
	        eventSystemFlags,
	        nativeEvent,
	        return_targetInst,
	        targetContainer
	      ),
	        clearIfContinuousEvent(domEventName, nativeEvent);
	    else if (
	      queueIfContinuousEvent(
	        blockedOn,
	        domEventName,
	        eventSystemFlags,
	        targetContainer,
	        nativeEvent
	      )
	    )
	      nativeEvent.stopPropagation();
	    else if (
	      (clearIfContinuousEvent(domEventName, nativeEvent),
	      eventSystemFlags & 4 &&
	        -1 < discreteReplayableEvents.indexOf(domEventName))
	    ) {
	      for (; null !== blockedOn; ) {
	        var fiber = getInstanceFromNode(blockedOn);
	        if (null !== fiber)
	          switch (fiber.tag) {
	            case 3:
	              fiber = fiber.stateNode;
	              if (fiber.current.memoizedState.isDehydrated) {
	                var lanes = getHighestPriorityLanes(fiber.pendingLanes);
	                if (0 !== lanes) {
	                  var root = fiber;
	                  root.pendingLanes |= 2;
	                  for (root.entangledLanes |= 2; lanes; ) {
	                    var lane = 1 << (31 - clz32(lanes));
	                    root.entanglements[1] |= lane;
	                    lanes &= ~lane;
	                  }
	                  ensureRootIsScheduled(fiber);
	                  0 === (executionContext & 6) &&
	                    ((workInProgressRootRenderTargetTime = now() + 500),
	                    flushSyncWorkAcrossRoots_impl(0, !1));
	                }
	              }
	              break;
	            case 31:
	            case 13:
	              (root = enqueueConcurrentRenderForLane(fiber, 2)),
	                null !== root && scheduleUpdateOnFiber(root, fiber, 2),
	                flushSyncWork$1(),
	                markRetryLaneIfNotHydrated(fiber, 2);
	          }
	        fiber = findInstanceBlockingEvent(nativeEvent);
	        null === fiber &&
	          dispatchEventForPluginEventSystem(
	            domEventName,
	            eventSystemFlags,
	            nativeEvent,
	            return_targetInst,
	            targetContainer
	          );
	        if (fiber === blockedOn) break;
	        blockedOn = fiber;
	      }
	      null !== blockedOn && nativeEvent.stopPropagation();
	    } else
	      dispatchEventForPluginEventSystem(
	        domEventName,
	        eventSystemFlags,
	        nativeEvent,
	        null,
	        targetContainer
	      );
	  }
	}
	function findInstanceBlockingEvent(nativeEvent) {
	  nativeEvent = getEventTarget(nativeEvent);
	  return findInstanceBlockingTarget(nativeEvent);
	}
	var return_targetInst = null;
	function findInstanceBlockingTarget(targetNode) {
	  return_targetInst = null;
	  targetNode = getClosestInstanceFromNode(targetNode);
	  if (null !== targetNode) {
	    var nearestMounted = getNearestMountedFiber(targetNode);
	    if (null === nearestMounted) targetNode = null;
	    else {
	      var tag = nearestMounted.tag;
	      if (13 === tag) {
	        targetNode = getSuspenseInstanceFromFiber(nearestMounted);
	        if (null !== targetNode) return targetNode;
	        targetNode = null;
	      } else if (31 === tag) {
	        targetNode = getActivityInstanceFromFiber(nearestMounted);
	        if (null !== targetNode) return targetNode;
	        targetNode = null;
	      } else if (3 === tag) {
	        if (nearestMounted.stateNode.current.memoizedState.isDehydrated)
	          return 3 === nearestMounted.tag
	            ? nearestMounted.stateNode.containerInfo
	            : null;
	        targetNode = null;
	      } else nearestMounted !== targetNode && (targetNode = null);
	    }
	  }
	  return_targetInst = targetNode;
	  return null;
	}
	function getEventPriority(domEventName) {
	  switch (domEventName) {
	    case "beforetoggle":
	    case "cancel":
	    case "click":
	    case "close":
	    case "contextmenu":
	    case "copy":
	    case "cut":
	    case "auxclick":
	    case "dblclick":
	    case "dragend":
	    case "dragstart":
	    case "drop":
	    case "focusin":
	    case "focusout":
	    case "input":
	    case "invalid":
	    case "keydown":
	    case "keypress":
	    case "keyup":
	    case "mousedown":
	    case "mouseup":
	    case "paste":
	    case "pause":
	    case "play":
	    case "pointercancel":
	    case "pointerdown":
	    case "pointerup":
	    case "ratechange":
	    case "reset":
	    case "resize":
	    case "seeked":
	    case "submit":
	    case "toggle":
	    case "touchcancel":
	    case "touchend":
	    case "touchstart":
	    case "volumechange":
	    case "change":
	    case "selectionchange":
	    case "textInput":
	    case "compositionstart":
	    case "compositionend":
	    case "compositionupdate":
	    case "beforeblur":
	    case "afterblur":
	    case "beforeinput":
	    case "blur":
	    case "fullscreenchange":
	    case "focus":
	    case "hashchange":
	    case "popstate":
	    case "select":
	    case "selectstart":
	      return 2;
	    case "drag":
	    case "dragenter":
	    case "dragexit":
	    case "dragleave":
	    case "dragover":
	    case "mousemove":
	    case "mouseout":
	    case "mouseover":
	    case "pointermove":
	    case "pointerout":
	    case "pointerover":
	    case "scroll":
	    case "touchmove":
	    case "wheel":
	    case "mouseenter":
	    case "mouseleave":
	    case "pointerenter":
	    case "pointerleave":
	      return 8;
	    case "message":
	      switch (getCurrentPriorityLevel()) {
	        case ImmediatePriority:
	          return 2;
	        case UserBlockingPriority:
	          return 8;
	        case NormalPriority$1:
	        case LowPriority:
	          return 32;
	        case IdlePriority:
	          return 268435456;
	        default:
	          return 32;
	      }
	    default:
	      return 32;
	  }
	}
	var hasScheduledReplayAttempt = !1,
	  queuedFocus = null,
	  queuedDrag = null,
	  queuedMouse = null,
	  queuedPointers = new Map(),
	  queuedPointerCaptures = new Map(),
	  queuedExplicitHydrationTargets = [],
	  discreteReplayableEvents =
	    "mousedown mouseup touchcancel touchend touchstart auxclick dblclick pointercancel pointerdown pointerup dragend dragstart drop compositionend compositionstart keydown keypress keyup input textInput copy cut paste click change contextmenu reset".split(
	      " "
	    );
	function clearIfContinuousEvent(domEventName, nativeEvent) {
	  switch (domEventName) {
	    case "focusin":
	    case "focusout":
	      queuedFocus = null;
	      break;
	    case "dragenter":
	    case "dragleave":
	      queuedDrag = null;
	      break;
	    case "mouseover":
	    case "mouseout":
	      queuedMouse = null;
	      break;
	    case "pointerover":
	    case "pointerout":
	      queuedPointers.delete(nativeEvent.pointerId);
	      break;
	    case "gotpointercapture":
	    case "lostpointercapture":
	      queuedPointerCaptures.delete(nativeEvent.pointerId);
	  }
	}
	function accumulateOrCreateContinuousQueuedReplayableEvent(
	  existingQueuedEvent,
	  blockedOn,
	  domEventName,
	  eventSystemFlags,
	  targetContainer,
	  nativeEvent
	) {
	  if (
	    null === existingQueuedEvent ||
	    existingQueuedEvent.nativeEvent !== nativeEvent
	  )
	    return (
	      (existingQueuedEvent = {
	        blockedOn: blockedOn,
	        domEventName: domEventName,
	        eventSystemFlags: eventSystemFlags,
	        nativeEvent: nativeEvent,
	        targetContainers: [targetContainer]
	      }),
	      null !== blockedOn &&
	        ((blockedOn = getInstanceFromNode(blockedOn)),
	        null !== blockedOn && attemptContinuousHydration(blockedOn)),
	      existingQueuedEvent
	    );
	  existingQueuedEvent.eventSystemFlags |= eventSystemFlags;
	  blockedOn = existingQueuedEvent.targetContainers;
	  null !== targetContainer &&
	    -1 === blockedOn.indexOf(targetContainer) &&
	    blockedOn.push(targetContainer);
	  return existingQueuedEvent;
	}
	function queueIfContinuousEvent(
	  blockedOn,
	  domEventName,
	  eventSystemFlags,
	  targetContainer,
	  nativeEvent
	) {
	  switch (domEventName) {
	    case "focusin":
	      return (
	        (queuedFocus = accumulateOrCreateContinuousQueuedReplayableEvent(
	          queuedFocus,
	          blockedOn,
	          domEventName,
	          eventSystemFlags,
	          targetContainer,
	          nativeEvent
	        )),
	        !0
	      );
	    case "dragenter":
	      return (
	        (queuedDrag = accumulateOrCreateContinuousQueuedReplayableEvent(
	          queuedDrag,
	          blockedOn,
	          domEventName,
	          eventSystemFlags,
	          targetContainer,
	          nativeEvent
	        )),
	        !0
	      );
	    case "mouseover":
	      return (
	        (queuedMouse = accumulateOrCreateContinuousQueuedReplayableEvent(
	          queuedMouse,
	          blockedOn,
	          domEventName,
	          eventSystemFlags,
	          targetContainer,
	          nativeEvent
	        )),
	        !0
	      );
	    case "pointerover":
	      var pointerId = nativeEvent.pointerId;
	      queuedPointers.set(
	        pointerId,
	        accumulateOrCreateContinuousQueuedReplayableEvent(
	          queuedPointers.get(pointerId) || null,
	          blockedOn,
	          domEventName,
	          eventSystemFlags,
	          targetContainer,
	          nativeEvent
	        )
	      );
	      return !0;
	    case "gotpointercapture":
	      return (
	        (pointerId = nativeEvent.pointerId),
	        queuedPointerCaptures.set(
	          pointerId,
	          accumulateOrCreateContinuousQueuedReplayableEvent(
	            queuedPointerCaptures.get(pointerId) || null,
	            blockedOn,
	            domEventName,
	            eventSystemFlags,
	            targetContainer,
	            nativeEvent
	          )
	        ),
	        !0
	      );
	  }
	  return !1;
	}
	function attemptExplicitHydrationTarget(queuedTarget) {
	  var targetInst = getClosestInstanceFromNode(queuedTarget.target);
	  if (null !== targetInst) {
	    var nearestMounted = getNearestMountedFiber(targetInst);
	    if (null !== nearestMounted)
	      if (((targetInst = nearestMounted.tag), 13 === targetInst)) {
	        if (
	          ((targetInst = getSuspenseInstanceFromFiber(nearestMounted)),
	          null !== targetInst)
	        ) {
	          queuedTarget.blockedOn = targetInst;
	          runWithPriority(queuedTarget.priority, function () {
	            attemptHydrationAtCurrentPriority(nearestMounted);
	          });
	          return;
	        }
	      } else if (31 === targetInst) {
	        if (
	          ((targetInst = getActivityInstanceFromFiber(nearestMounted)),
	          null !== targetInst)
	        ) {
	          queuedTarget.blockedOn = targetInst;
	          runWithPriority(queuedTarget.priority, function () {
	            attemptHydrationAtCurrentPriority(nearestMounted);
	          });
	          return;
	        }
	      } else if (
	        3 === targetInst &&
	        nearestMounted.stateNode.current.memoizedState.isDehydrated
	      ) {
	        queuedTarget.blockedOn =
	          3 === nearestMounted.tag
	            ? nearestMounted.stateNode.containerInfo
	            : null;
	        return;
	      }
	  }
	  queuedTarget.blockedOn = null;
	}
	function attemptReplayContinuousQueuedEvent(queuedEvent) {
	  if (null !== queuedEvent.blockedOn) return !1;
	  for (
	    var targetContainers = queuedEvent.targetContainers;
	    0 < targetContainers.length;

	  ) {
	    var nextBlockedOn = findInstanceBlockingEvent(queuedEvent.nativeEvent);
	    if (null === nextBlockedOn) {
	      nextBlockedOn = queuedEvent.nativeEvent;
	      var nativeEventClone = new nextBlockedOn.constructor(
	        nextBlockedOn.type,
	        nextBlockedOn
	      );
	      currentReplayingEvent = nativeEventClone;
	      nextBlockedOn.target.dispatchEvent(nativeEventClone);
	      currentReplayingEvent = null;
	    } else
	      return (
	        (targetContainers = getInstanceFromNode(nextBlockedOn)),
	        null !== targetContainers &&
	          attemptContinuousHydration(targetContainers),
	        (queuedEvent.blockedOn = nextBlockedOn),
	        !1
	      );
	    targetContainers.shift();
	  }
	  return !0;
	}
	function attemptReplayContinuousQueuedEventInMap(queuedEvent, key, map) {
	  attemptReplayContinuousQueuedEvent(queuedEvent) && map.delete(key);
	}
	function replayUnblockedEvents() {
	  hasScheduledReplayAttempt = !1;
	  null !== queuedFocus &&
	    attemptReplayContinuousQueuedEvent(queuedFocus) &&
	    (queuedFocus = null);
	  null !== queuedDrag &&
	    attemptReplayContinuousQueuedEvent(queuedDrag) &&
	    (queuedDrag = null);
	  null !== queuedMouse &&
	    attemptReplayContinuousQueuedEvent(queuedMouse) &&
	    (queuedMouse = null);
	  queuedPointers.forEach(attemptReplayContinuousQueuedEventInMap);
	  queuedPointerCaptures.forEach(attemptReplayContinuousQueuedEventInMap);
	}
	function scheduleCallbackIfUnblocked(queuedEvent, unblocked) {
	  queuedEvent.blockedOn === unblocked &&
	    ((queuedEvent.blockedOn = null),
	    hasScheduledReplayAttempt ||
	      ((hasScheduledReplayAttempt = !0),
	      Scheduler.unstable_scheduleCallback(
	        Scheduler.unstable_NormalPriority,
	        replayUnblockedEvents
	      )));
	}
	var lastScheduledReplayQueue = null;
	function scheduleReplayQueueIfNeeded(formReplayingQueue) {
	  lastScheduledReplayQueue !== formReplayingQueue &&
	    ((lastScheduledReplayQueue = formReplayingQueue),
	    Scheduler.unstable_scheduleCallback(
	      Scheduler.unstable_NormalPriority,
	      function () {
	        lastScheduledReplayQueue === formReplayingQueue &&
	          (lastScheduledReplayQueue = null);
	        for (var i = 0; i < formReplayingQueue.length; i += 3) {
	          var form = formReplayingQueue[i],
	            submitterOrAction = formReplayingQueue[i + 1],
	            formData = formReplayingQueue[i + 2];
	          if ("function" !== typeof submitterOrAction)
	            if (null === findInstanceBlockingTarget(submitterOrAction || form))
	              continue;
	            else break;
	          var formInst = getInstanceFromNode(form);
	          null !== formInst &&
	            (formReplayingQueue.splice(i, 3),
	            (i -= 3),
	            startHostTransition(
	              formInst,
	              {
	                pending: !0,
	                data: formData,
	                method: form.method,
	                action: submitterOrAction
	              },
	              submitterOrAction,
	              formData
	            ));
	        }
	      }
	    ));
	}
	function retryIfBlockedOn(unblocked) {
	  function unblock(queuedEvent) {
	    return scheduleCallbackIfUnblocked(queuedEvent, unblocked);
	  }
	  null !== queuedFocus && scheduleCallbackIfUnblocked(queuedFocus, unblocked);
	  null !== queuedDrag && scheduleCallbackIfUnblocked(queuedDrag, unblocked);
	  null !== queuedMouse && scheduleCallbackIfUnblocked(queuedMouse, unblocked);
	  queuedPointers.forEach(unblock);
	  queuedPointerCaptures.forEach(unblock);
	  for (var i = 0; i < queuedExplicitHydrationTargets.length; i++) {
	    var queuedTarget = queuedExplicitHydrationTargets[i];
	    queuedTarget.blockedOn === unblocked && (queuedTarget.blockedOn = null);
	  }
	  for (
	    ;
	    0 < queuedExplicitHydrationTargets.length &&
	    ((i = queuedExplicitHydrationTargets[0]), null === i.blockedOn);

	  )
	    attemptExplicitHydrationTarget(i),
	      null === i.blockedOn && queuedExplicitHydrationTargets.shift();
	  i = (unblocked.ownerDocument || unblocked).$$reactFormReplay;
	  if (null != i)
	    for (queuedTarget = 0; queuedTarget < i.length; queuedTarget += 3) {
	      var form = i[queuedTarget],
	        submitterOrAction = i[queuedTarget + 1],
	        formProps = form[internalPropsKey] || null;
	      if ("function" === typeof submitterOrAction)
	        formProps || scheduleReplayQueueIfNeeded(i);
	      else if (formProps) {
	        var action = null;
	        if (submitterOrAction && submitterOrAction.hasAttribute("formAction"))
	          if (
	            ((form = submitterOrAction),
	            (formProps = submitterOrAction[internalPropsKey] || null))
	          )
	            action = formProps.formAction;
	          else {
	            if (null !== findInstanceBlockingTarget(form)) continue;
	          }
	        else action = formProps.action;
	        "function" === typeof action
	          ? (i[queuedTarget + 1] = action)
	          : (i.splice(queuedTarget, 3), (queuedTarget -= 3));
	        scheduleReplayQueueIfNeeded(i);
	      }
	    }
	}
	function defaultOnDefaultTransitionIndicator() {
	  function handleNavigate(event) {
	    event.canIntercept &&
	      "react-transition" === event.info &&
	      event.intercept({
	        handler: function () {
	          return new Promise(function (resolve) {
	            return (pendingResolve = resolve);
	          });
	        },
	        focusReset: "manual",
	        scroll: "manual"
	      });
	  }
	  function handleNavigateComplete() {
	    null !== pendingResolve && (pendingResolve(), (pendingResolve = null));
	    isCancelled || setTimeout(startFakeNavigation, 20);
	  }
	  function startFakeNavigation() {
	    if (!isCancelled && !navigation.transition) {
	      var currentEntry = navigation.currentEntry;
	      currentEntry &&
	        null != currentEntry.url &&
	        navigation.navigate(currentEntry.url, {
	          state: currentEntry.getState(),
	          info: "react-transition",
	          history: "replace"
	        });
	    }
	  }
	  if ("object" === typeof navigation) {
	    var isCancelled = !1,
	      pendingResolve = null;
	    navigation.addEventListener("navigate", handleNavigate);
	    navigation.addEventListener("navigatesuccess", handleNavigateComplete);
	    navigation.addEventListener("navigateerror", handleNavigateComplete);
	    setTimeout(startFakeNavigation, 100);
	    return function () {
	      isCancelled = !0;
	      navigation.removeEventListener("navigate", handleNavigate);
	      navigation.removeEventListener("navigatesuccess", handleNavigateComplete);
	      navigation.removeEventListener("navigateerror", handleNavigateComplete);
	      null !== pendingResolve && (pendingResolve(), (pendingResolve = null));
	    };
	  }
	}
	function ReactDOMRoot(internalRoot) {
	  this._internalRoot = internalRoot;
	}
	ReactDOMHydrationRoot.prototype.render = ReactDOMRoot.prototype.render =
	  function (children) {
	    var root = this._internalRoot;
	    if (null === root) throw Error(formatProdErrorMessage(409));
	    var current = root.current,
	      lane = requestUpdateLane();
	    updateContainerImpl(current, lane, children, root, null, null);
	  };
	ReactDOMHydrationRoot.prototype.unmount = ReactDOMRoot.prototype.unmount =
	  function () {
	    var root = this._internalRoot;
	    if (null !== root) {
	      this._internalRoot = null;
	      var container = root.containerInfo;
	      updateContainerImpl(root.current, 2, null, root, null, null);
	      flushSyncWork$1();
	      container[internalContainerInstanceKey] = null;
	    }
	  };
	function ReactDOMHydrationRoot(internalRoot) {
	  this._internalRoot = internalRoot;
	}
	ReactDOMHydrationRoot.prototype.unstable_scheduleHydration = function (target) {
	  if (target) {
	    var updatePriority = resolveUpdatePriority();
	    target = { blockedOn: null, target: target, priority: updatePriority };
	    for (
	      var i = 0;
	      i < queuedExplicitHydrationTargets.length &&
	      0 !== updatePriority &&
	      updatePriority < queuedExplicitHydrationTargets[i].priority;
	      i++
	    );
	    queuedExplicitHydrationTargets.splice(i, 0, target);
	    0 === i && attemptExplicitHydrationTarget(target);
	  }
	};
	var isomorphicReactPackageVersion$jscomp$inline_1840 = React.version;
	if (
	  "19.2.3" !==
	  isomorphicReactPackageVersion$jscomp$inline_1840
	)
	  throw Error(
	    formatProdErrorMessage(
	      527,
	      isomorphicReactPackageVersion$jscomp$inline_1840,
	      "19.2.3"
	    )
	  );
	ReactDOMSharedInternals.findDOMNode = function (componentOrElement) {
	  var fiber = componentOrElement._reactInternals;
	  if (void 0 === fiber) {
	    if ("function" === typeof componentOrElement.render)
	      throw Error(formatProdErrorMessage(188));
	    componentOrElement = Object.keys(componentOrElement).join(",");
	    throw Error(formatProdErrorMessage(268, componentOrElement));
	  }
	  componentOrElement = findCurrentFiberUsingSlowPath(fiber);
	  componentOrElement =
	    null !== componentOrElement
	      ? findCurrentHostFiberImpl(componentOrElement)
	      : null;
	  componentOrElement =
	    null === componentOrElement ? null : componentOrElement.stateNode;
	  return componentOrElement;
	};
	var internals$jscomp$inline_2347 = {
	  bundleType: 0,
	  version: "19.2.3",
	  rendererPackageName: "react-dom",
	  currentDispatcherRef: ReactSharedInternals,
	  reconcilerVersion: "19.2.3"
	};
	if ("undefined" !== typeof __REACT_DEVTOOLS_GLOBAL_HOOK__) {
	  var hook$jscomp$inline_2348 = __REACT_DEVTOOLS_GLOBAL_HOOK__;
	  if (
	    !hook$jscomp$inline_2348.isDisabled &&
	    hook$jscomp$inline_2348.supportsFiber
	  )
	    try {
	      (rendererID = hook$jscomp$inline_2348.inject(
	        internals$jscomp$inline_2347
	      )),
	        (injectedHook = hook$jscomp$inline_2348);
	    } catch (err) {}
	}
	reactDomClient_production.createRoot = function (container, options) {
	  if (!isValidContainer(container)) throw Error(formatProdErrorMessage(299));
	  var isStrictMode = !1,
	    identifierPrefix = "",
	    onUncaughtError = defaultOnUncaughtError,
	    onCaughtError = defaultOnCaughtError,
	    onRecoverableError = defaultOnRecoverableError;
	  null !== options &&
	    void 0 !== options &&
	    (!0 === options.unstable_strictMode && (isStrictMode = !0),
	    void 0 !== options.identifierPrefix &&
	      (identifierPrefix = options.identifierPrefix),
	    void 0 !== options.onUncaughtError &&
	      (onUncaughtError = options.onUncaughtError),
	    void 0 !== options.onCaughtError && (onCaughtError = options.onCaughtError),
	    void 0 !== options.onRecoverableError &&
	      (onRecoverableError = options.onRecoverableError));
	  options = createFiberRoot(
	    container,
	    1,
	    !1,
	    null,
	    null,
	    isStrictMode,
	    identifierPrefix,
	    null,
	    onUncaughtError,
	    onCaughtError,
	    onRecoverableError,
	    defaultOnDefaultTransitionIndicator
	  );
	  container[internalContainerInstanceKey] = options.current;
	  listenToAllSupportedEvents(container);
	  return new ReactDOMRoot(options);
	};
	reactDomClient_production.hydrateRoot = function (container, initialChildren, options) {
	  if (!isValidContainer(container)) throw Error(formatProdErrorMessage(299));
	  var isStrictMode = !1,
	    identifierPrefix = "",
	    onUncaughtError = defaultOnUncaughtError,
	    onCaughtError = defaultOnCaughtError,
	    onRecoverableError = defaultOnRecoverableError,
	    formState = null;
	  null !== options &&
	    void 0 !== options &&
	    (!0 === options.unstable_strictMode && (isStrictMode = !0),
	    void 0 !== options.identifierPrefix &&
	      (identifierPrefix = options.identifierPrefix),
	    void 0 !== options.onUncaughtError &&
	      (onUncaughtError = options.onUncaughtError),
	    void 0 !== options.onCaughtError && (onCaughtError = options.onCaughtError),
	    void 0 !== options.onRecoverableError &&
	      (onRecoverableError = options.onRecoverableError),
	    void 0 !== options.formState && (formState = options.formState));
	  initialChildren = createFiberRoot(
	    container,
	    1,
	    !0,
	    initialChildren,
	    null != options ? options : null,
	    isStrictMode,
	    identifierPrefix,
	    formState,
	    onUncaughtError,
	    onCaughtError,
	    onRecoverableError,
	    defaultOnDefaultTransitionIndicator
	  );
	  initialChildren.context = getContextForSubtree(null);
	  options = initialChildren.current;
	  isStrictMode = requestUpdateLane();
	  isStrictMode = getBumpedLaneForHydrationByLane(isStrictMode);
	  identifierPrefix = createUpdate(isStrictMode);
	  identifierPrefix.callback = null;
	  enqueueUpdate(options, identifierPrefix, isStrictMode);
	  options = isStrictMode;
	  initialChildren.current.lanes = options;
	  markRootUpdated$1(initialChildren, options);
	  ensureRootIsScheduled(initialChildren);
	  container[internalContainerInstanceKey] = initialChildren.current;
	  listenToAllSupportedEvents(container);
	  return new ReactDOMHydrationRoot(initialChildren);
	};
	reactDomClient_production.version = "19.2.3";
	return reactDomClient_production;
}

var client$1 = client$2.exports;

var hasRequiredClient;

function requireClient () {
	if (hasRequiredClient) return client$2.exports;
	hasRequiredClient = 1;
	"use strict";
	function checkDCE() {
	  if (typeof __REACT_DEVTOOLS_GLOBAL_HOOK__ === "undefined" || typeof __REACT_DEVTOOLS_GLOBAL_HOOK__.checkDCE !== "function") {
	    return;
	  }
	  if (false) {
	    throw new Error("^_^");
	  }
	  try {
	    __REACT_DEVTOOLS_GLOBAL_HOOK__.checkDCE(checkDCE);
	  } catch (err) {
	    console.error(err);
	  }
	}
	if (true) {
	  checkDCE();
	  client$2.exports = requireReactDomClient_production();
	} else {
	  module.exports = require("./cjs/react-dom-client.development.js");
	}
	return client$2.exports;
}

var clientExports = requireClient();
const client = /*@__PURE__*/getDefaultExportFromCjs(clientExports);

beforeMount(async ({ App }) => {
  return /* @__PURE__ */ jsxRuntimeExports.jsx(BrowserRouter, { children: /* @__PURE__ */ jsxRuntimeExports.jsx(SettingsProvider, { children: /* @__PURE__ */ jsxRuntimeExports.jsx(App, {}) }) });
});


"use strict";

// packages/playwright-ct-core/src/injected/importRegistry.ts
function isImportRef(value) {
  return typeof value === "object" && value && value.__pw_type === "importRef";
}
var ImportRegistry = class {
  constructor() {
    this._registry = /* @__PURE__ */ new Map();
  }
  initialize(components) {
    for (const [name, value] of Object.entries(components))
      this._registry.set(name, value);
  }
  async resolveImportRef(importRef) {
    const importFunction = this._registry.get(importRef.id);
    if (!importFunction)
      throw new Error(`Unregistered component: ${importRef.id}. Following components are registered: ${[...this._registry.keys()]}`);
    let importedObject = await importFunction();
    if (!importedObject)
      throw new Error(`Could not resolve component: ${importRef.id}.`);
    if (importRef.property) {
      importedObject = importedObject[importRef.property];
      if (!importedObject)
        throw new Error(`Could not instantiate component: ${importRef.id}.${importRef.property}.`);
    }
    return importedObject;
  }
};

// packages/playwright-ct-core/src/injected/serializers.ts
function isFunctionRef(value) {
  return value && typeof value === "object" && value.__pw_type === "function";
}
async function unwrapObject(value) {
  return transformObjectAsync(value, async (v) => {
    if (isFunctionRef(v)) {
      const result = (...args) => {
        window.__ctDispatchFunction(v.ordinal, args);
      };
      return { result };
    }
    if (isImportRef(v))
      return { result: await window.__pwRegistry.resolveImportRef(v) };
  });
}
function transformObject(value, mapping) {
  const result = mapping(value);
  if (result)
    return result.result;
  if (value === null || typeof value !== "object")
    return value;
  if (value instanceof Date || value instanceof RegExp || value instanceof URL)
    return value;
  if (Array.isArray(value)) {
    const result3 = [];
    for (const item of value)
      result3.push(transformObject(item, mapping));
    return result3;
  }
  if ((value == null ? void 0 : value.__pw_type) === "jsx" && typeof value.type === "function") {
    throw new Error([
      `Component "${value.type.name}" cannot be mounted.`,
      `Most likely, this component is defined in the test file. Create a test story instead.`,
      `For more information, see https://playwright.dev/docs/test-components#test-stories.`
    ].join("\n"));
  }
  const result2 = {};
  for (const [key, prop] of Object.entries(value))
    result2[key] = transformObject(prop, mapping);
  return result2;
}
async function transformObjectAsync(value, mapping) {
  const result = await mapping(value);
  if (result)
    return result.result;
  if (value === null || typeof value !== "object")
    return value;
  if (value instanceof Date || value instanceof RegExp || value instanceof URL)
    return value;
  if (Array.isArray(value)) {
    const result3 = [];
    for (const item of value)
      result3.push(await transformObjectAsync(item, mapping));
    return result3;
  }
  const result2 = {};
  for (const [key, prop] of Object.entries(value))
    result2[key] = await transformObjectAsync(prop, mapping);
  return result2;
}

// packages/playwright-ct-core/src/injected/index.ts
window.__pwRegistry = new ImportRegistry();
window.__pwUnwrapObject = unwrapObject;
window.__pwTransformObject = transformObject;
/** @typedef {import('../playwright-ct-core/types/component').JsxComponent} JsxComponent */

/** @type {Map<Element, { root: import('react-dom/client').Root, setRenderer: (renderer: any) => void }>} */
const __pwRootRegistry = new Map();

/**
 * @param {any} component
 * @returns {component is JsxComponent}
 */
function isJsxComponent(component) {
  return typeof component === 'object' && component && component.__pw_type === 'jsx';
}

/**
 * @param {any} type
 * @returns {boolean} type is Playwright's mock JSX.Fragment
 */
function isJsxFragment(type) {
  return typeof type === 'object' && type?.__pw_jsx_fragment;
}

/**
 * Turns the Playwright representation of JSX (see jsx-runtime.js) into React.createElement calls.
 * @param {any} value
 */
function __pwRender(value) {
  return window.__pwTransformObject(value, v => {
    if (isJsxComponent(v)) {
      const component = v;
      let type = component.type;
      if (isJsxFragment(type))
        type = React.Fragment;
      const props = component.props ? __pwRender(component.props) : {};
      const key = component.key ? __pwRender(component.key) : undefined;
      const { children, ...propsWithoutChildren } = props;
      if (key)
        propsWithoutChildren.key = key;
      const createElementArguments = [propsWithoutChildren];
      if (children)
        createElementArguments.push(children);
      return { result: React.createElement(type, ...createElementArguments) };
    }
  });
}

window.playwrightMount = async (component, rootElement, hooksConfig) => {
  if (!isJsxComponent(component))
    throw new Error('Object mount notation is not supported');
  if (__pwRootRegistry.has(rootElement)) {
    throw new Error(
        'Attempting to mount a component into an container that already has a React root'
    );
  }

  const root = clientExports.createRoot(rootElement);
  const entry = { root, setRenderer: () => undefined };
  __pwRootRegistry.set(rootElement, entry);

  const App = () => {
    /** @type {any} */
    const [renderer, setRenderer] = React.useState(() => __pwRender(component));
    entry.setRenderer = setRenderer;
    return renderer;
  };
  let AppWrapper = App;
  for (const hook of window.__pw_hooks_before_mount || []) {
    const wrapper = await hook({ App: AppWrapper, hooksConfig });
    if (wrapper)
      AppWrapper = () => wrapper;
  }

  root.render(React.createElement(AppWrapper));

  for (const hook of window.__pw_hooks_after_mount || [])
    await hook({ hooksConfig });
};

window.playwrightUnmount = async rootElement => {
  const entry = __pwRootRegistry.get(rootElement);
  if (!entry)
    throw new Error('Component was not mounted');

  entry.root.unmount();
  __pwRootRegistry.delete(rootElement);
};

window.playwrightUpdate = async (rootElement, component) => {
  if (!isJsxComponent(component))
    throw new Error('Object mount notation is not supported');

  const entry = __pwRootRegistry.get(rootElement);
  if (!entry)
    throw new Error('Component was not mounted');
  entry.setRenderer(() => __pwRender(component));
};

const _home_moritz_dev_spectre_via_ssh_ui_src_App = () => __vitePreload(() => import('./App-CN82QFaE.js'),true              ?__vite__mapDeps([0,1]):void 0).then((mod) => mod.default);
const _home_moritz_dev_spectre_via_ssh_ui_src_pages_SettingsPage = () => __vitePreload(() => import('./SettingsPage-DlyVciZE.js').then(n => n.c),true              ?[]:void 0).then((mod) => mod.default);
__pwRegistry.initialize({ _home_moritz_dev_spectre_via_ssh_ui_src_App,
  _home_moritz_dev_spectre_via_ssh_ui_src_pages_SettingsPage });

export { COMMON_KINDS as C, DEFAULT_KINDS as D, NavLink as N, React as R, __vitePreload as _, useSearchParams as a, Routes as b, Route as c, ReactDOM as d, jsxRuntimeExports as j, reactExports as r, useSettings as u };
//# sourceMappingURL=index-rD0dd41P.js.map
