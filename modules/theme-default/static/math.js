document.addEventListener("DOMContentLoaded", function () {
    document.querySelectorAll(".math-inline").forEach(function (el) {
        var tex = el.textContent;
        try {
            el.innerHTML = katex.renderToString(tex, { throwOnError: false });
        } catch (e) {}
    });
    document.querySelectorAll(".math-block").forEach(function (el) {
        var tex = el.textContent;
        try {
            el.innerHTML = katex.renderToString(tex, { displayMode: true, throwOnError: false });
        } catch (e) {}
    });
});
