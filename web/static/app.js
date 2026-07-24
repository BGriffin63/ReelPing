// ReelPing — minimal progressive-enhancement script. No external dependencies.
(function () {
  "use strict";

  // Apply saved/explicit theme early is handled inline; here we wire the toggle.
  function applyTheme(t) {
    if (t === "light" || t === "dark") {
      document.documentElement.setAttribute("data-theme", t);
    } else {
      document.documentElement.removeAttribute("data-theme");
    }
    try { localStorage.setItem("rp-theme", t); } catch (e) {}
  }
  var stored = null;
  try { stored = localStorage.getItem("rp-theme"); } catch (e) {}
  if (stored) applyTheme(stored);

  document.addEventListener("click", function (e) {
    var t = e.target.closest("[data-theme-set]");
    if (t) { e.preventDefault(); applyTheme(t.getAttribute("data-theme-set")); }
  });

  // Password strength meter (setup + change password).
  function scorePassword(pw) {
    if (!pw) return { score: 0, label: "" };
    var score = 0;
    if (pw.length >= 12) score++;
    if (pw.length >= 16) score++;
    if (pw.length >= 24) score++;
    var classes = 0;
    if (/[a-z]/.test(pw)) classes++;
    if (/[A-Z]/.test(pw)) classes++;
    if (/[0-9]/.test(pw)) classes++;
    if (/[^A-Za-z0-9]/.test(pw)) classes++;
    if (classes >= 3) score++;
    if (score > 4) score = 4;
    var labels = ["Very weak", "Weak", "Fair", "Good", "Strong"];
    var colors = ["#d64542", "#d64542", "#d69e0d", "#e08a1e", "#1f9d57"];
    return { score: score, label: labels[score], color: colors[score] };
  }
  var pwInput = document.querySelector("[data-pw-meter]");
  if (pwInput) {
    var bar = document.querySelector("[data-pw-bar] > span");
    var lbl = document.querySelector("[data-pw-label]");
    pwInput.addEventListener("input", function () {
      var r = scorePassword(pwInput.value);
      if (bar) { bar.style.width = ((r.score / 4) * 100) + "%"; bar.style.background = r.color || "#d64542"; }
      if (lbl) lbl.textContent = r.label;
    });
  }

  // Confirm destructive actions.
  document.addEventListener("submit", function (e) {
    var f = e.target;
    var msg = f.getAttribute("data-confirm");
    if (msg && !window.confirm(msg)) { e.preventDefault(); return; }
    // Prevent double submission (belt-and-braces; server also uses idempotency).
    var btn = f.querySelector('button[type=submit], input[type=submit]');
    if (btn && f.getAttribute("data-no-double") !== "off") {
      setTimeout(function () { btn.disabled = true; btn.textContent = btn.getAttribute("data-busy") || "Working…"; }, 0);
    }
  });

  // Async connection tests (Plex / Discord) with a small result area.
  document.querySelectorAll("[data-test-endpoint]").forEach(function (btn) {
    btn.addEventListener("click", function () {
      var endpoint = btn.getAttribute("data-test-endpoint");
      var out = document.querySelector(btn.getAttribute("data-test-output"));
      var form = btn.closest("form");
      var body = form ? new URLSearchParams(new FormData(form)) : new URLSearchParams();
      if (out) { out.textContent = "Testing…"; out.className = "flash info"; }
      fetch(endpoint, { method: "POST", body: body, headers: { "X-Requested-With": "fetch" } })
        .then(function (r) { return r.json(); })
        .then(function (d) {
          if (out) { out.textContent = d.message || (d.ok ? "Success" : "Failed"); out.className = "flash " + (d.ok ? "ok" : "err"); }
        })
        .catch(function () { if (out) { out.textContent = "Request failed."; out.className = "flash err"; } });
    });
  });
})();
