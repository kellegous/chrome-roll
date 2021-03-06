(function(){
#include "../common.js"

var AUTHOR_ALIASES = {
  'commit-queue@webkit.org' : 'Commit Bot'
};

// todo: this is a terrible mess.
function reveal(q, element) {
  var h = element.getBoundingClientRect().height;
  element.css('height', '0px').css('overflow', 'hidden');
  q.push(function() {
    element.transition('height', h + 'px', function() {
      q.shift();
      element.style.removeProperty('overflow');
      next = q[0];
      if (next)
        next();
    });
  });
  if (q.length == 1) {
    setTimeout(function() {
      q[0]();
    }, 0);
  }
}

function updateText(element, text) {
  var q = element._q || (element._q = []);
  q.push(function() {
    element.transition('opacity', '0', function() {
      q.shift();
      element.text('' + text).transition('opacity', '1', q[0]);
    });
  });
  if (q.length == 1)
    q[0]();
}

// SvgKitten
function SvgKitten() {
  var call = document.create('div');
  call.className = 'call';
  var msgt = document.create('div').text('svg kitten says:');
  msgt.className = 'title';
  var msgb = document.create('div');
  msgb.className = 'message';
  
  var says = document.create('div').add(call, msgt, msgb);
  says.id = 'svgkitten-says';
  
  var root = document.create('div').add(says);
  root.id = 'svgkitten';

  document.body.add(root);

  this._says = says;
  this._root = root;
  this._mesg = msgb;
  this.hide();
}

SvgKitten.prototype.show = function(message, showDidFinish) {
  this._mesg.text(message);
  this._root.transition('opacity', '1', showDidFinish);
  return this;
}
SvgKitten.prototype.hide = function(hideDidFinish) {
  var root = this._root;
  this._root.transition('opacity', '0', hideDidFinish);
}

// Model
function Model(listener) {
  this._listener = listener;
  this._reset();
}

Model.prototype.kittens = function() {
  return this._kittens;
}

Model.prototype.kittenChangeCount = function() {
  var c = 0;
  this._kittens.forEach(function(kitten) {
    c += kitten.Revisions.length;
  });
  return c;
}

Model.prototype.subscribe = function(kitten, callback) {
  var entry = this._index[kitten.Email];
  if (!entry)
    return false;
  entry.Callbacks.push(callback);
  return true;
}

Model.prototype._reset = function() {
  this._index = {};
  this._kittens = [];
}

Model.prototype._dispatch = function(name, args) {
  var listener = this._listener && this._listener[name];
  if (listener)
    listener.apply(null, args);
}

Model.prototype.messageDidArrive = function(m) {
  var self = this;
  switch (m.Type) {
  case "connect":
    self._reset();
    // Populate the model.
    var kittens = m.Kittens;
    kittens.forEach(function(k) {
      self._kittens.push(k);
      self._index[k.Email] = {
        Kitten: k,
        Callbacks: []
      };
    });

    // Dispatch the load event.
    self._dispatch('modelDidLoad', [self, m.Changes, m.Version]);
    break;
  case "change":
    var change = m.Change;
    var kittens = m.Kittens;

    // update model state
    var toCallback = [];
    kittens.forEach(function(email) {
      var entry = self._index[email];
      if (!entry)
        return;
      entry.Kitten.Revisions.push(change.Revision);
      toCallback.push(entry);
    });

    // now dispatch callbacks
    kittens.forEach(function(email) {
      var entry = self._index[email];
      if (!entry)
        return;
      self._dispatch('kittenDidMakeChange', [self, entry.Kitten, {}]);
    });
    toCallback.forEach(function(entry) {
      entry.Callbacks.forEach(function(cb) {
        cb(entry.Kitten);
      });
    });

    self._dispatch('changeDidArrive', [change, kittens]);
    break;
  }
}

var MS_IN_SECOND = 1000;

Model.connect = function(path, listener) {

  function newSocket(url, model, reconnectIn) {
    function nextTimeout(current) {
      return (current >= 30 * MS_IN_SECOND) ? current : current * 2;
    }
    var socket = new WebSocket(url);
    socket.onopen = function() {
      model._dispatch('socketDidOpen', [model]);
    }
    socket.onclose = function() {
      socket.onopen = null;
      socket.onmessage = null;
      socket.onclose = null;
      socket.close();
      model._dispatch('socketDidClose', [model]);
      setTimeout(function() {
        newSocket(url, model, nextTimeout(reconnectIn));
      }, reconnectIn);
    }
    socket.onmessage = function(m) {
      model.messageDidArrive(JSON.parse(m.data));
    }
  }

  var basepath = window.location.pathname;
  if (!basepath.endsWith('/'))
    basepath += '/';
  if (path.startsWith('/'))
    path = path.substring(1);
  var url = 'ws://' + window.location.host + basepath + path;

  var model = new Model(listener);
  newSocket(url, model, 1000);
}

/** @return string */
function formatTime(date) {
  function p(n) { return n = n.toFixed(), n.length == 1 ? '0' + n : n; }
  return p(date.getHours()) + ':' + p(date.getMinutes());
}

/**
 * @constructor View
 */
function View(model) {

  /** @returns string */
  function usernameOf(kitten) {
    var email = kitten.Email;
    var ix = email.indexOf('@');
    return ix < 0 ? email : email.substring(0, ix);
  }

  /** @returns Object */
  function boundsOf(elements) {
    var b;
    elements.forEach(function(e) {
      var box = e.getBoundingClientRect();
      if (!b)
        b = { left: box.left, right: box.right, top: box.top, bottom: box.bottom };
      b.left = Math.min(b.left, box.left);
      b.right = Math.max(b.right, box.right);
      b.top = Math.min(b.top, box.top);
      b.bottom = Math.max(b.bottom, box.bottom);
    });
    return b;
  }

  /** @returns Element */
  function newKittenView(model, kitten) {
    var text = document.create('div').cls('text');
    kitten.Name.split(' ').forEach(function(x) {
      text.add(document.create('div').text(x));
    });
    var badge = document.create('div').cls('badge')
        .text('' + kitten.Revisions.length);
    model.subscribe(kitten, function(kitten) {
      updateText(badge, kitten.Revisions.length);
    });
    return document.create('div').cls('kitten')
      .css('background-image', 'url(img/' +  usernameOf(kitten) + '.png)')
      .add(text, badge);
  }

  /** @returns Element */
  function newKittensView(root) {
    var e = document.create('div').cls('kittens');
    root.add(e);
    return e;
  }

  /** @returns void */
  function createKittensViews(root, model) {
    var kittens = model.kittens();
    var v = newKittensView(root);
    kittens.forEach(function(kitten) {
      if (v.qa('.kitten').length >= 5)
        v = newKittensView(root);
      v.add(newKittenView(model, kitten));
    });
  }

  // get reference to overall count badge.
  var badgeView = document.qo('#badge-count').text('' + model.kittenChangeCount());

  // get reference to root.
  var rootView = document.qo('#root');

  // build 0 or more kittens views.
  createKittensViews(root, model);

  // create changes view.
  var changesView = document.create('div').attr('id', 'changes');
  rootView.add(changesView);

  rootView.css('opacity', '1.0');
  // Scale the UI to the size of the monitor.
  var bounds = boundsOf(document.qa('#team > *'));
  var scale = 0.9 * window.innerWidth / (bounds.right - bounds.left);
  document.body.css('zoom', scale)
    .css('overflow', 'hidden');

  // add shortcut key for enabling scrollbars.
  document.addEventListener('keypress', function(e) {
    var b = document.body;
    if (e.ctrlKey && e.shiftKey && e.keyCode == 19 /* s */) {
      if (b.style.overflow === 'hidden') {
        b.css('overflow', 'visible');
      } else {
        b.css('overflow', 'hidden');
        b.scrollTop = 0;
      }
    }
  }, true);

  // create references to elements needed later.
  this._badgeView = badgeView;
  this._rootView = rootView;
  this._changesView = changesView;
}
View.prototype.destroy = function() {
  this._rootView.style.removeProperty('-webkit-transform');
  this._rootView.qa('.kittens').forEach(function(e) {
    e.remove();
  });
  this._changesView.remove();
}
View.prototype.beMeek = function(v) {
  this._rootView.css('opacity', v ? '0.5' : '1.0');
}
View.prototype.kittenDidMakeChange = function(model, kitten, change) {
  updateText(this._badgeView, model.kittenChangeCount());
}
View.prototype.sleepyStateDidChange = function(beSleepy) {
  // todo: just turn the screen black right now, but we need something
  // more entertaining.
  var sleepy = document.querySelector('#sleepy');
  if (!sleepy)
    return;
  sleepy.style.opacity = beSleepy ? '1' : '0';
}
View.prototype.changeDidArrive = function(change, loadInProgress) {
  function formatCommentAsHtml(comment) {
    var result = [];
    var lines = comment.split('\n');
    for (var i = 0; i < lines.length; ++i) {
      var line = lines[i];
      var text = line.trim();
      if (text.startsWith('*'))
        break;
      result.push(text.length == 0 ? '' : line);
    }
    return result.join('\n');
  }
  function formatTitle(rev, author) {
    var name = AUTHOR_ALIASES[author] || author;
    return 'r' + rev + ' by ' + name;
  }
  var changesView = this._changesView;
  var c = document.create('div').cls('change').add(
      document.create('div').cls('title').add(
        document.create('span').text(formatTitle(change.Revision, change.Author)),
        document.create('span').cls('time').text(formatTime(new Date(Date.parse(change.Date))))),
      document.create('div').cls('comment').text(formatCommentAsHtml(change.Comment)));
  changesView.prepend(c);
  if (!loadInProgress)
    reveal(changesView._q || (changesView._q = []), c);
  while (changesView.childElementCount > 20)
    changesView.removeChild(changesView.lastElementChild);
}

function main() {
  var view, serverVersion;
  var svgKitten = new SvgKitten();

  Model.connect('str', {
    modelDidLoad: function(model, changes, version) {
      // on reconnect, we want to reload if the server changed.
      if (serverVersion && serverVersion != version) {
        location.reload(true);
        return;
      }
      serverVersion = version;

      // We also want to completely restore the view.
      if (view)
        view.destroy();
      view = new View(model);

      for (var i = changes.length - 1; i >= 0; --i) {
        view.changeDidArrive(changes[i], true);
      }
    },
    changeDidArrive: function(change, kittens) {
      // todo: get rid of these callbacks that have to be
      // repushed into the view.
      view.changeDidArrive(change, false);
    },
    kittenDidMakeChange: function(model, kitten, change) {
      // todo: get rid of these callbacks that have to be
      // repushed into the view.
      view.kittenDidMakeChange(model, kitten, change);
    },
    socketDidOpen: function(model) {
      svgKitten.hide();
      if (view)
        view.beMeek(false);
    },
    socketDidClose: function(model) {
      svgKitten.show('your server broke!');
      if (view)
        view.beMeek(true);
    }
  });

  var inSleepyTime = false;
  setInterval(function() {
    if (!view)
      return;
    var h = new Date().getHours();
    // only show between 7 & 7.
    var isSleepyTime = h >= 19 || h <= 7;
    if (isSleepyTime != inSleepyTime) {
      view.sleepyStateDidChange(isSleepyTime);
      inSleepyTime = isSleepyTime;
    }
  }, 10000);
}

whenReady(main);
})();
