(function(){

const SVGNS = "http://www.w3.org/2000/svg";

function xhrGet(url, callback, errback) {
  var xhr = new XMLHttpRequest;
  xhr.onreadystatechange = function() {
    if (xhr.readyState == 4) {
      if (xhr.status == 200)
        callback && callback(xhr);
      else
        errback && errback(xhr);
    }
  }
  xhr.open('GET', url, true);
  xhr.send(null);
}

SVGSVGElement.prototype.sizeToContent = function() {
  var r = Rect.fromRect(this.getBoundingClientRect());
  var els = this.querySelectorAll('*');
  for (var i = 0, n = els.length; i < n; ++i)
    r = r.union(els[i].getBoundingClientRect());
  this.attr('width', r.width).attr('height', r.height);
}

Element.prototype.attr = function(n, v) {
  this.setAttribute(n, v);
  return this;
}

Document.prototype.newSvg = function(name) {
  return this.createElementNS(SVGNS, name);
}

function Rect(top, left, bottom, right) {
  this.top = top;
  this.left = left;
  this.bottom = bottom;
  this.right = right;
  this.width = this.right - this.left;
  this.height = this.bottom - this.top;
}
Rect.empty = function() {
  return new Rect(0, 0, 0, 0);
}
Rect.fromRect = function(r) {
  return new Rect(r.top, r.left, r.bottom, r.right);
}

Rect.prototype.union = function(r) {
  return new Rect(
    Math.min(this.top, r.top),
    Math.min(this.left, r.left),
    Math.max(this.bottom, r.bottom),
    Math.max(this.right, r.right));
}

function computeBoundingBox(root) {
  var rct = new Rect();
  var els = root.querySelectorAll('*');
  for (var i = 0, n = els.length; i < n; ++i) {
    var el = els[i];
    rct = rct.union(el.getBoundingClientRect());
  }
  return rct;
}

function createBox(x, y, w, h) {
  return document.newSvg('rect')
      .attr('x', x)
      .attr('y', y)
      .attr('width', w)
      .attr('height', h);
}

function dataDidArrive(data) {
  var root = document.documentElement;
  for (var i = 0; i < data.length; ++i) {
    var y = i * 55;
    root.appendChild(createBox(10, y, 200, 50).attr('style', 'fill:red;opacity:0.5;'));
  }
  root.sizeToContent();
}

function domDidLoad() {
  xhrGet('/chrome/?l=25',
    function(xhr) {
      dataDidArrive(JSON.parse(xhr.responseText));
    },
    function(xhr) {
    });
}

addEventListener('DOMContentLoaded', domDidLoad, false);
})()
