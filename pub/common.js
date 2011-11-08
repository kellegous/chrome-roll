function whenReady(f) {
  if (document.readyState == 'complete') {
    f();
    return;
  }
  window.addEventListener('DOMContentLoaded', f, false);
}
Document.prototype.create = function(name) {
  return this.createElement(name);
}
Node.prototype.qo = function(selector) {
  return this.querySelector(selector);
}
Node.prototype.qa = function(selector) {
  return this.querySelectorAll(selector);
}
Node.prototype.attr = function(k, v) {
  this.setAttribute(k, v);
  return this;
}
Node.prototype.css = function(n, v) {
  this.style.setProperty(n, v);
  return this;
}
Node.prototype.text = function(v) {
  this.textContent = v;
  return this;
}
Node.prototype.add = function() {
  for (var i = 0, n = arguments.length; i < n; ++i)
    this.appendChild(arguments[i]);
  return this;
}
Node.prototype.update = function(callback) {
  callback(this);
  return this;
}
NodeList.prototype.forEach = function(f) {
  for (var i = 0, n = this.length; i < n; ++i)
    f(this[i]);
}
NodeList.prototype.map = function(f) {
  var r = [];
  for (var i = 0, n = this.length; i < n; ++i)
    r.push(f(this[i]));
  return r;
}
