/**
 * @file 主入口文件 - 初始化和启动
 * 
 * 依赖加载顺序:
 * 1. mitt.umd.js - 事件库
 * 2. eventbus.js - 事件总线 (WXE)
 * 3. utils.js - 工具函数 (WXU)
 * 4. core.js - 核心工具
 * 5. decrypt.js - 解密模块
 * 6. download.js - 下载模块
 * 7. home.js - Home页面模块
 * 8. main.js - 本文件，初始化入口
 */
console.log('[main.js] 主入口脚本开始加载...');

// ==================== 初始化 ====================
(function() {
  console.log('[main.js] 开始初始化...');
  
  // 延迟执行，等待DOM和其他脚本加载完成
  setTimeout(function() {
    console.log('[main.js] 开始执行 insert_download_btn');
    
    // 检查必要的函数是否存在
    if (typeof insert_download_btn === 'function') {
      insert_download_btn();
    } else {
      console.error('[main.js] insert_download_btn 函数未定义');
    }
  }, 800);
})();

console.log('[main.js] 主入口脚本加载完成');
