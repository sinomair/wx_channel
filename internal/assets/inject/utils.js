/**
 * @file 工具函数 - 参考 wx_channels_download 项目
 */
var __wx_channels_tip__ = {};
var __wx_channels_cur_video = null;
var __wx_channels_store__ = {
  profile: null,
  buffers: [],
  keys: {},
};

function __wx_channels_video_decrypt(t, e, p) {
  for (var r = new Uint8Array(t), n = 0; n < t.byteLength && e + n < p.decryptor_array.length; n++)
    r[n] ^= p.decryptor_array[n];
  return r;
}

window.VTS_WASM_URL = "https://res.wx.qq.com/t/wx_fed/cdn_libs/res/decrypt-video-core/1.3.0/wasm_video_decode.wasm";
window.MAX_HEAP_SIZE = 33554432;
var decryptor_array;
let decryptor;

function wasm_isaac_generate(t, e) {
  decryptor_array = new Uint8Array(e);
  var r = new Uint8Array(Module.HEAPU8.buffer, t, e);
  decryptor_array.set(r.reverse());
  if (decryptor) decryptor.delete();
}

let loaded = false;
const __decrypt_cache__ = new Map();

async function __wx_channels_decrypt(seed) {
  const cacheKey = String(seed);
  if (__decrypt_cache__.has(cacheKey)) return __decrypt_cache__.get(cacheKey);
  if (!loaded) {
    await WXU.load_script("https://res.wx.qq.com/t/wx_fed/cdn_libs/res/decrypt-video-core/1.3.0/wasm_video_decode.js");
    loaded = true;
  }
  await WXU.sleep();
  decryptor = new Module.WxIsaac64(seed);
  decryptor.generate(131072);
  const result = new Uint8Array(decryptor_array);
  __decrypt_cache__.set(cacheKey, result);
  return result;
}

var WXU = (() => {
  var defaultRandomAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789";
  
  // API 对象
  var WXAPI = {};
  var WXAPI2 = {};
  
  // 监听 APILoaded 事件，保存 API 函数
  WXE.onAPILoaded((variables) => {
    console.log('[WXU] APILoaded 事件触发，变量:', variables);
    const keys = Object.keys(variables);
    for (let i = 0; i < keys.length; i++) {
      (() => {
        const variable = keys[i];
        const methods = variables[variable];
        // 检查是否包含 finderGetCommentDetail 函数（API 组）
        if (typeof methods.finderGetCommentDetail === "function") {
          WXAPI = methods;
          console.log('[WXU] ✅ WXAPI 已初始化，包含函数:', Object.keys(methods).slice(0, 10));
          return;
        }
        // 检查是否包含 finderSearch 函数（API2 组）
        if (typeof methods.finderSearch === "function") {
          WXAPI2 = methods;
          console.log('[WXU] ✅ WXAPI2 已初始化，包含函数:', Object.keys(methods).slice(0, 10));
          return;
        }
      })();
    }
  });
  
  function __wx_uid__() { return random_string(12); }
  function random_string(length) { return random_string_with_alphabet(length, defaultRandomAlphabet); }
  function random_string_with_alphabet(length, alphabet) {
    let b = new Array(length);
    let max = alphabet.length;
    for (let i = 0; i < b.length; i++) {
      let n = Math.floor(Math.random() * max);
      b[i] = alphabet[n];
    }
    return b.join("");
  }
  function sleep(ms) {
    return new Promise((resolve) => { setTimeout(() => { resolve(); }, ms || 1000); });
  }

  // 清理 HTML 标签
  function clean_html_tags(text) {
    if (!text || typeof text !== 'string') return text || '';
    var tempDiv = document.createElement('div');
    tempDiv.innerHTML = text;
    var cleaned = tempDiv.textContent || tempDiv.innerText || '';
    return cleaned.trim();
  }

  function format_feed(feed) {
    // 处理正在直播的数据（liveStatus === 1 表示正在直播）
    if (feed.liveInfo && feed.liveInfo.liveStatus === 1) {
      // 正在直播，返回直播类型数据
      var liveTitle = feed.liveInfo.description || (feed.objectDesc && feed.objectDesc.description) || '直播中';
      return {
        ...feed,
        type: "live",
        id: feed.id,
        nonce_id: feed.objectNonceId,
        title: clean_html_tags(liveTitle),
        coverUrl: feed.liveInfo.coverUrl || (feed.objectDesc && feed.objectDesc.media && feed.objectDesc.media[0] && feed.objectDesc.media[0].thumbUrl) || '',
        thumbUrl: feed.liveInfo.coverUrl || '',
        nickname: feed.contact ? feed.contact.nickname : '',
        contact: feed.contact || {},
        createtime: feed.createtime || 0,
        liveInfo: feed.liveInfo,
        // 直播暂时不能下载
        canDownload: false
      };
    }
    if (!feed.objectDesc) return null;
    var type = feed.objectDesc.mediaType;
    if (type === 9) return null;
    var media = feed.objectDesc.media && feed.objectDesc.media[0];
    if (!media) return null;
    if (type === 2) {
      return {
        ...feed,
        type: "picture",
        id: feed.id,
        nonce_id: feed.objectNonceId,
        cover_url: media.coverUrl,
        title: clean_html_tags(feed.objectDesc.description),
        files: feed.objectDesc.media,
        spec: [],
        contact: feed.contact ? {
          id: feed.contact.username,
          avatar_url: feed.contact.headUrl,
          nickname: feed.contact.nickname,
        } : null,
      };
    }
    if (type === 4) {
      // 获取时长（毫秒）：优先使用 spec[0].durationMs，其次使用 videoPlayLen * 1000
      var duration = 0;
      if (media.spec && media.spec.length > 0 && media.spec[0].durationMs) {
        duration = media.spec[0].durationMs;
      } else if (media.videoPlayLen) {
        // videoPlayLen 单位是秒，需要转换为毫秒
        duration = media.videoPlayLen * 1000;
      }
      
      return {
        ...feed,
        type: "media",
        id: feed.id,
        nonce_id: feed.objectNonceId,
        title: clean_html_tags(feed.objectDesc.description),
        url: media.url + media.urlToken,
        key: media.decodeKey,
        cover_url: media.coverUrl,
        coverUrl: media.thumbUrl,
        thumbUrl: media.thumbUrl,
        fullThumbUrl: media.fullThumbUrl,
        createtime: feed.createtime,
        spec: media.spec,
        size: media.fileSize,
        duration: duration,
        media: media,
        contact: feed.contact ? {
          id: feed.contact.username,
          avatar_url: feed.contact.headUrl,
          nickname: feed.contact.nickname,
        } : null,
        nickname: feed.contact ? feed.contact.nickname : "",
        readCount: feed.readCount,
        likeCount: feed.likeCount,
        commentCount: feed.commentCount,
        favCount: feed.favCount,
        forwardCount: feed.forwardCount,
        ipRegionInfo: feed.ipRegionInfo,
        // 视频可以下载
        canDownload: true
      };
    }
    return null;
  }

  function __wx_channels_copy(text) {
    var textArea = document.createElement("textarea");
    textArea.value = text;
    textArea.style.cssText = "position: absolute; top: -999px; left: -999px;";
    document.body.appendChild(textArea);
    textArea.select();
    document.execCommand("copy");
    document.body.removeChild(textArea);
  }

  function __wx_log(params) {
    console.log("[log]", params);
    fetch("/__wx_channels_api/tip", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(params),
    });
  }

  const script_loaded_map = {};
  function __wx_load_script(src) {
    const existing = script_loaded_map[src];
    if (existing) return existing;
    const p = new Promise((resolve, reject) => {
      const script = document.createElement("script");
      script.type = "text/javascript";
      script.src = src;
      script.onload = resolve;
      script.onerror = reject;
      document.head.appendChild(script);
    });
    script_loaded_map[src] = p;
    return p;
  }

  function __wx_find_elm(selector) {
    return new Promise((resolve) => {
      var __count = 0;
      var __timer = setInterval(() => {
        __count += 1;
        var $elm = selector();
        if (!$elm) {
          if (__count >= 5) {
            clearInterval(__timer);
            __timer = null;
            resolve(null);
          }
          return;
        }
        resolve($elm);
        clearInterval(__timer);
      }, 200);
    });
  }

  return {
    ...WXE,
    sleep,
    uid: __wx_uid__,
    load_script: __wx_load_script,
    find_elm: __wx_find_elm,
    copy: __wx_channels_copy,
    log: __wx_log,
    // API 对象的 getter
    get API() {
      return WXAPI;
    },
    get API2() {
      return WXAPI2;
    },
    format_feed,
    build_decrypt_arr: __wx_channels_decrypt,
    video_decrypt: __wx_channels_video_decrypt,
    async decrypt_video(buf, key) {
      try {
        const r = await __wx_channels_decrypt(key);
        if (r) {
          buf = __wx_channels_video_decrypt(buf, 0, { decryptor_array: r });
          return [null, buf];
        }
        return [new Error("前端解密失败"), null];
      } catch (err) {
        return [err, null];
      }
    },
    set_cur_video() {
      setTimeout(() => {
        window.__wx_channels_cur_video = document.querySelector(".feed-video.video-js");
      }, 800);
    },
    set_feed(feed) {
      var profile = format_feed(feed);
      if (!profile) return;
      console.log("[WXU.set_feed] 发送profile到后端", profile.title);
      fetch("/__wx_channels_api/profile", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(profile),
      });
      __wx_channels_store__.profile = profile;
      fetch("/__wx_channels_api/tip", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ msg: "📹 " + (profile.nickname || "未知作者") + " - " + (profile.title || "").substring(0, 30) + "..." })
      }).catch(function() {});
    },
    check_feed_existing(opt) {
      opt = opt || {};
      var profile = __wx_channels_store__.profile;
      if (!profile) {
        if (!opt.silence) {
          alert("检测不到视频，请将本工具更新到最新版");
        }
        return [true, null];
      }
      return [false, profile];
    },
    // 解析URL查询参数
    get_queries(href) {
      var parts = decodeURIComponent(href).split("?");
      if (parts.length < 2) return {};
      var search = parts[1];
      var queries = decodeURIComponent(search)
        .split("&")
        .map(function(item) {
          var kv = item.split("=");
          var obj = {};
          obj[kv[0]] = kv[1];
          return obj;
        })
        .reduce(function(prev, cur) {
          for (var k in cur) {
            prev[k] = cur[k];
          }
          return prev;
        }, {});
      return queries;
    },
    // 监听DOM节点出现
    observe_node(selector, cb) {
      var $existing = document.querySelector(selector);
      if ($existing) {
        cb($existing);
        return;
      }
      var observer = new MutationObserver(function(mutations, obs) {
        mutations.forEach(function(mutation) {
          if (mutation.type === "childList") {
            mutation.addedNodes.forEach(function(node) {
              if (node.nodeType === 1) {
                if (node.matches && node.matches(selector)) {
                  cb(node);
                  obs.disconnect();
                } else if (node.querySelector) {
                  var found = node.querySelector(selector);
                  if (found) {
                    cb(found);
                    obs.disconnect();
                  }
                }
              }
            });
          }
        });
      });
      // 等待页面加载后开始观察
      var startObserve = function() {
        var app = document.getElementById("app") || document.body;
        observer.observe(app, {
          childList: true,
          subtree: true,
        });
      };
      if (document.readyState === "complete") {
        startObserve();
      } else {
        window.addEventListener("load", startObserve);
      }
    },
    // 显示toast提示
    toast(msg, duration) {
      duration = duration || 2000;
      var $toast = document.createElement("div");
      $toast.className = "wx-channels-toast";
      $toast.innerText = msg;
      $toast.style.cssText = "position:fixed;top:50%;left:50%;transform:translate(-50%,-50%);background:rgba(0,0,0,0.7);color:#fff;padding:12px 24px;border-radius:8px;z-index:99999;font-size:14px;";
      document.body.appendChild($toast);
      setTimeout(function() {
        $toast.remove();
      }, duration);
    },
    // 显示错误提示
    error(opt) {
      opt = opt || {};
      console.error("[WXU.error]", opt.msg);
      __wx_log({ msg: "❌ " + opt.msg });
      if (opt.alert !== 0) {
        alert(opt.msg);
      }
    },
  };
})();
