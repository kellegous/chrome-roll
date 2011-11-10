(function(){
#include "../common.js"

// Model
function Model(socket, listener) {
  this._socket = socket;
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

Model.connect = function(path, listener) {
  var basepath = window.location.pathname;
  if (!basepath.endsWith('/'))
    basepath += '/';
  if (path.startsWith('/'))
    path = path.substring(1);
  var url = 'ws://' + window.location.host + basepath + path;

  var socket = new WebSocket(url);
  var model = new Model(socket, listener);

  socket.onopen = function() {
    console.log('socket is open');
  }
  socket.onclose = function() {
    // TODO: reconnect and possibly refresh.
    console.log('socket is closed');
  }
  socket.onmessage = function(m) {
    model.messageDidArrive(JSON.parse(m.data));
  }
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
  Model.connect('str', {
    modelDidLoad: function(model, changes) {
      destroyUi();
      createUi(model);
    },
    changeDidArrive: function(change, kittens) {
      console.log(change);
    },
    kittenMadeChange: function(model, kitten, change) {
      document.qo('#badge-count').text('' + model.kittenChangeCount());
    }
  });
}

whenReady(main);

})();
