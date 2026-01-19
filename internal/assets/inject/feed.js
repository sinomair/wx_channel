/**
 * @file Feed页面功能模块 - 视频详情页下载按钮注入
 */
console.log('[feed.js] 加载Feed页面模块');

// ==================== Feed页面下载按钮注入 ====================

/** 注入Feed页面顶部工具栏按钮 */
async function __insert_download_btn_to_feed_toolbar() {
  // 查找顶部工具栏容器
  var findToolbarContainer = function() {
    // 尝试多种选择器
    var container = document.querySelector('div[data-v-bf57a568].flex.items-center');
    if (container) return container;
    
    var parent = document.querySelector('div.flex-initial.flex-shrink-0.pl-6');
    if (parent) {
      container = parent.querySelector('.flex.items-center');
      if (container) return container;
    }
    
    // 尝试查找包含相机图标的容器
    var cameraIcon = document.querySelector('svg[data-v-bf57a568]');
    if (cameraIcon) {
      var current = cameraIcon;
      while (current && current.parentElement) {
        current = current.parentElement;
        if (current.classList && current.classList.contains('flex') && current.classList.contains('items-center')) {
          return current;
        }
      }
    }
    
    return null;
  };

  var tryInject = function() {
    var container = findToolbarContainer();
    if (!container) return false;
    
    // 检查是否已存在
    if (container.querySelector('#wx-feed-comment-icon') || container.querySelector('#wx-feed-download-icon')) {
      console.log('[feed.js] 工具栏按钮已存在');
      return true;
    }

    // 创建评论图标
    var commentIconWrapper = document.createElement('div');
    commentIconWrapper.id = 'wx-feed-comment-icon';
    commentIconWrapper.className = 'mr-4 h-6 w-6 flex-initial flex-shrink-0 text-fg-0 cursor-pointer';
    commentIconWrapper.title = '采集评论';
    commentIconWrapper.innerHTML = '<svg class="h-full w-full" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none"><path fill-rule="evenodd" clip-rule="evenodd" d="M4 4C3.44772 4 3 4.44772 3 5V15C3 15.5523 3.44772 16 4 16H7V19L11 16H20C20.5523 16 21 15.5523 21 15V5C21 4.44772 20.5523 4 20 4H4ZM1 5C1 3.34315 2.34315 2 4 2H20C21.6569 2 23 3.34315 23 5V15C23 16.6569 21.6569 18 20 18H11.618L7.30902 21.0257C6.81409 21.3605 6.13993 21.2361 5.78169 20.7639C5.60436 20.5341 5.5 20.2471 5.5 19.9506V18H4C2.34315 18 1 16.6569 1 15V5Z" fill="currentColor"></path></svg>';
    
    commentIconWrapper.onclick = function() {
      if (window.__wx_channels_start_comment_collection) {
        window.__wx_channels_start_comment_collection();
      }
    };

    // 创建下载图标
    var downloadIconWrapper = document.createElement('div');
    downloadIconWrapper.id = 'wx-feed-download-icon';
    downloadIconWrapper.className = 'mr-4 h-6 w-6 flex-initial flex-shrink-0 text-fg-0 cursor-pointer';
    downloadIconWrapper.title = '下载视频';
    downloadIconWrapper.innerHTML = '<svg class="h-full w-full" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none"><path fill-rule="evenodd" clip-rule="evenodd" d="M12 3C12.3314 3 12.6 3.26863 12.6 3.6V13.1515L15.5757 10.1757C15.8101 9.94142 16.1899 9.94142 16.4243 10.1757C16.6586 10.4101 16.6586 10.7899 16.4243 11.0243L12.4243 15.0243C12.1899 15.2586 11.8101 15.2586 11.5757 15.0243L7.57574 11.0243C7.34142 10.7899 7.34142 10.4101 7.57574 10.1757C7.81005 9.94142 8.18995 9.94142 8.42426 10.1757L11.4 13.1515V3.6C11.4 3.26863 11.6686 3 12 3ZM3.6 14.4C3.93137 14.4 4.2 14.6686 4.2 15V19.2C4.2 19.5314 4.46863 19.8 4.8 19.8H19.2C19.5314 19.8 19.8 19.5314 19.8 19.2V15C19.8 14.6686 20.0686 14.4 20.4 14.4C20.7314 14.4 21 14.6686 21 15V19.2C21 20.1941 20.1941 21 19.2 21H4.8C3.80589 21 3 20.1941 3 19.2V15C3 14.6686 3.26863 14.4 3.6 14.4Z" fill="currentColor"></path></svg>';
    
    downloadIconWrapper.onclick = function() {
      __handle_feed_download_click();
    };

    // 插入到容器最前面
    container.insertBefore(downloadIconWrapper, container.firstChild);
    container.insertBefore(commentIconWrapper, container.firstChild);
    
    console.log('[feed.js] ✅ 工具栏按钮注入成功');
    __wx_log({ msg: "注入评论和下载按钮成功!" });
    return true;
  };

  // 立即尝试注入
  if (tryInject()) return true;

  // 如果失败，使用 MutationObserver 监听 DOM 变化
  return new Promise(function(resolve) {
    var observer = new MutationObserver(function(mutations, obs) {
      if (tryInject()) {
        obs.disconnect();
        resolve(true);
      }
    });
    
    observer.observe(document.body, { 
      childList: true, 
      subtree: true 
    });
    
    // 5秒后超时
    setTimeout(function() {
      observer.disconnect();
      console.log('[feed.js] 工具栏按钮注入超时');
      resolve(false);
    }, 5000);
  });
}

/** Feed页面下载按钮点击处理 */
function __handle_feed_download_click() {
  var profile = window.__wx_channels_store__ && window.__wx_channels_store__.profile;
  
  if (!profile) {
    __wx_log({ msg: '⏳ 正在获取视频数据，请稍候...' });
    
    // 等待数据
    var checkCount = 0;
    var maxChecks = 30;
    var checkData = function() {
      profile = window.__wx_channels_store__ && window.__wx_channels_store__.profile;
      if (profile) {
        __show_feed_download_options(profile);
      } else {
        checkCount++;
        if (checkCount < maxChecks) {
          setTimeout(checkData, 100);
        } else {
          __wx_log({ msg: '❌ 获取视频数据超时\n请刷新页面重试' });
        }
      }
    };
    checkData();
    return;
  }
  
  __show_feed_download_options(profile);
}

/** Feed页面下载选项菜单 */
function __show_feed_download_options(profile) {
  console.log('[feed.js] 显示下载选项菜单', profile);
  
  // 移除已存在的菜单
  var existingMenu = document.getElementById('wx-download-menu');
  if (existingMenu) existingMenu.remove();
  var existingOverlay = document.getElementById('wx-download-overlay');
  if (existingOverlay) existingOverlay.remove();
  
  var menu = document.createElement('div');
  menu.id = 'wx-download-menu';
  menu.style.cssText = 'position:fixed;top:60px;right:20px;z-index:99999;background:#2b2b2b;color:#e5e5e5;border-radius:8px;padding:0;width:280px;box-shadow:0 8px 24px rgba(0,0,0,0.5);font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,"Helvetica Neue",Arial,sans-serif;font-size:14px;';
  
  var title = profile.title || '未知视频';
  var shortTitle = title.length > 30 ? title.substring(0, 30) + '...' : title;
  
  var html = '';
  
  // 标题栏
  html += '<div style="padding:16px 20px;border-bottom:1px solid rgba(255,255,255,0.08);">';
  html += '<div style="font-size:15px;font-weight:500;color:#fff;margin-bottom:8px;">下载选项</div>';
  html += '<div style="font-size:13px;color:#999;line-height:1.4;">' + shortTitle + '</div>';
  html += '</div>';
  
  // 选项区域
  html += '<div style="padding:16px 20px;">';
  
  // 视频下载选项
  if (profile.spec && profile.spec.length > 0) {
    html += '<div style="margin-bottom:12px;font-size:12px;color:#999;">选择画质:</div>';
    profile.spec.forEach(function(spec, index) {
      var label = spec.fileFormat || ('画质' + (index + 1));
      if (spec.width && spec.height) {
        label += ' (' + spec.width + 'x' + spec.height + ')';
      }
      html += '<div class="download-option" data-index="' + index + '" style="padding:10px 16px;margin:8px 0;background:rgba(255,255,255,0.08);border-radius:6px;cursor:pointer;text-align:center;transition:background 0.2s;font-size:13px;">' + label + '</div>';
    });
  } else {
    html += '<div class="download-option" data-index="-1" style="padding:10px 16px;margin:8px 0;background:rgba(255,255,255,0.08);border-radius:6px;cursor:pointer;text-align:center;font-size:13px;">下载视频</div>';
  }
  
  // 封面下载
  html += '<div class="download-cover" style="padding:10px 16px;margin:8px 0;background:rgba(7,193,96,0.15);color:#07c160;border-radius:6px;cursor:pointer;text-align:center;font-size:13px;font-weight:500;">下载封面</div>';
  
  html += '</div>';
  
  // 底部按钮
  html += '<div style="padding:12px 20px;border-top:1px solid rgba(255,255,255,0.08);">';
  html += '<div class="close-menu" style="padding:8px;text-align:center;cursor:pointer;color:#999;font-size:13px;">取消</div>';
  html += '</div>';
  
  menu.innerHTML = html;
  document.body.appendChild(menu);
  
  // 添加遮罩
  var overlay = document.createElement('div');
  overlay.id = 'wx-download-overlay';
  overlay.style.cssText = 'position:fixed;top:0;left:0;right:0;bottom:0;background:rgba(0,0,0,0.5);z-index:99998;';
  document.body.appendChild(overlay);
  
  function closeMenu() {
    menu.remove();
    overlay.remove();
  }
  
  // 绑定事件
  menu.querySelectorAll('.download-option').forEach(function(el) {
    el.onmouseover = function() { this.style.background = 'rgba(255,255,255,0.15)'; };
    el.onmouseout = function() { this.style.background = 'rgba(255,255,255,0.08)'; };
    el.onclick = function() {
      var index = parseInt(this.getAttribute('data-index'));
      var spec = index >= 0 && profile.spec ? profile.spec[index] : null;
      closeMenu();
      __wx_channels_handle_click_download__(spec);
    };
  });
  
  var coverBtn = menu.querySelector('.download-cover');
  coverBtn.onmouseover = function() { this.style.background = 'rgba(7,193,96,0.25)'; };
  coverBtn.onmouseout = function() { this.style.background = 'rgba(7,193,96,0.15)'; };
  coverBtn.onclick = function() {
    closeMenu();
    __wx_channels_handle_download_cover();
  };
  
  menu.querySelector('.close-menu').onclick = closeMenu;
  overlay.onclick = closeMenu;
}

/** Feed页面按钮注入入口 */
async function __insert_download_btn_to_feed_page() {
  console.log('[feed.js] 开始注入Feed页面按钮到顶部工具栏...');
  
  var success = await __insert_download_btn_to_feed_toolbar();
  if (success) return true;
  
  console.log('[feed.js] 未找到Feed页面工具栏');
  return false;
}

console.log('[feed.js] Feed页面模块加载完成');
