(function(){
#include "../common.js"

// a makeshift transition queue.
function changeBadge(element, count) {
  var q = element._q || (element._q = []);
  q.push(function() {
    element.transition('opacity', '0', function() {
      q.shift();
      element.text('' + count).transition('opacity', '1', q[0]);
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

function newKittensView(root) {
  var e = document.create('div');
  e.className = 'kittens';
  e.isFull = function() {
    return e.qa('.kitten').length >= 5;
  }
  root.add(e);
  return e;
}

function newKittenView(model, kitten) {
  function usernameOf(kitten) {
    var email = kitten.Email;
    var ix = email.indexOf('@');
    return ix < 0 ? email : email.substring(0, ix);
  }

  var root = document.create('div');
  root.className = 'kitten';

  var text = document.create('div');
  kitten.Name.split(' ').forEach(function(x) {
    text.add(document.create('div').text(x));
  });
  text.className = 'text';

  var badge = document.create('div').text('' + kitten.Revisions.length);
  badge.className = 'badge';

  model.subscribe(kitten, function(kitten) {
    changeBadge(badge, kitten.Revisions.length);
  });
  // makeshift transition queue.
  var q = [];
  model.subscribe(kitten, function(kitten) {
    // capture the count because it may change.
    var count = kitten.Revisions.length;
    q.push(function() {
      badge.transition('opacity', '0', function() {
        q.shift();
        badge.text('' + count).transition('opacity', '1', q[0]);
      });
    });
    if (q.length == 1)
      q[0]();
  });

  return root
    .css('background-image', 'url(img/' +  usernameOf(kitten) + '.png)')
    .add(text, badge);

}

function createUi(model) {
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

  var badgeCount = document.qo('#badge-count').text('' + model.kittenChangeCount());

  var kittens = model.kittens();
  if (kittens.length == 0)
    return;

  var root = document.qo('#root');
  var kittensView = newKittensView(root);
  kittens.forEach(function(kitten) {
    if (kittensView.isFull())
      kittensView = newKittensView(root);
    kittensView.add(newKittenView(model, kitten));
  });

  // Scale the UI to the size of the monitor.
  var bounds = boundsOf(document.qa('#team > *'));
  var scale = 0.9 * window.innerWidth / (bounds.right - bounds.left);
  root.css('-webkit-transform', 'scale(' + scale + ' ,' + scale  + ')');
}

function destroyUi() {
  document.body.qo('#root').style.removeProperty('-webkit-transform');
  document.body.qa('.kittens').forEach(function(e) {
    e.remove();
  });
}

function main() {
  var serverVersion;
  var svgKitten = new SvgKitten();
  Model.connect('str', {
    modelDidLoad: function(model, changes, version) {
      // on reconnect, we want to reload if the server changed.
      if (serverVersion && serverVersion != version) {
        location.reload(true);
        return;
      }
      serverVersion = version;
      destroyUi();
      createUi(model);
      document.body.css('opacity', '1.0');
    },
    changeDidArrive: function(change, kittens) {
      console.log(change);
    },
    kittenDidMakeChange: function(model, kitten, change) {
      console.log(change);
      changeBadge(document.qo('#badge-count'), model.kittenChangeCount());
    },
    socketDidOpen: function(model) {
      svgKitten.hide();
    },
    socketDidClose: function(model) {
      svgKitten.show('your server broke!');
    }
  });
}

whenReady(main);
})();
