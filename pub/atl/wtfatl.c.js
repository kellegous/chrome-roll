(function(){
#include "../common.js"

function Tx(element) {
  this._element = element;
}
Tx.prototype.then = function(f) {
  var e = this._element;
  var cb = function() {
    e.removeEventListener('webkitTransitionEnd', cb, false);
    f();
  }
  e.addEventListener('webkitTransitionEnd', cb, false);
  return this;
}

// SvgKitten
function SvgKitten() {
  var call = document.create('div');
  call.className = 'call';
  var msgt = document.create('div').text('svn kitten says:');
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

SvgKitten.prototype.show = function(message) {
  this._mesg.text(message);
  return new Tx(this._root.css('opacity', '1.0'));
}
SvgKitten.prototype.hide = function() {
  var root = this._root;
  return new Tx(root.css('opacity', '0.0'));
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
    self._dispatch('modelDidLoad', [self, m.Changes]);
    break;
  case "change":
    var change = m.Change;
    var kittens = m.Kittens;

    var toCallback = [];
    kittens.forEach(function(email) {
      var entry = self._index[email];
      if (!entry)
        return;
      entry.Kitten.Revisions.push(change.Revision);
      toCallback.push(entry);
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

var MS_IN_MINUTE = 60 * 1000;

Model.connect = function(path, listener) {

  function newSocket(url, model, reconnectIn) {
    function nextTimeout(current) {
      return (current >= 10 * MS_IN_MINUTE) ? current : current * 2;
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

function newKittensView() {
  var e = document.create('div');
  e.className = 'kittens';
  e.isFull = function() {
    return e.qa('.kitten').length >= 5;
  }
  document.body.add(e);
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
    badge.text('' + kitten.Revisions.length);
  });

  return root
    .css('background-image', 'url(img/' +  usernameOf(kitten) + '.png)')
    .add(text, badge);

}

function createUi(model) {
  var badgeCount = document.qo('#badge-count').text('' + model.kittenChangeCount());

  var kittens = model.kittens();
  if (kittens.length == 0)
    return;

  var kittensView = newKittensView();
  kittens.forEach(function(kitten) {
    if (kittensView.isFull())
      kittensView = newKittensView();
    kittensView.add(newKittenView(model, kitten));
  });
}

function destroyUi() {
  document.body.qa('.kittens').forEach(function(e) {
    e.remove();
  });
}

function main() {
  var svgKitten = new SvgKitten();
  var svgKitten;
  Model.connect('str', {
    modelDidLoad: function(model, changes) {
      destroyUi();
      createUi(model);
      document.body.css('opacity', '1.0');
    },
    changeDidArrive: function(change, kittens) {
      console.log(change);
    },
    kittenMadeChange: function(model, kitten, change) {
      console.log(change);
      document.qo('#badge-count').text('' + model.kittenChangeCount());
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
