// 工时记录 - 全局 JS

// 自动隐藏 flash 消息
document.addEventListener('DOMContentLoaded', function() {
    const flashes = document.querySelectorAll('.flash');
    flashes.forEach(function(el) {
        setTimeout(function() {
            el.style.transition = 'opacity .5s';
            el.style.opacity = '0';
            setTimeout(function() { el.remove(); }, 500);
        }, 3000);
    });
});
