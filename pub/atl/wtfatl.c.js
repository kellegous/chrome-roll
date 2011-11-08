(function(){
#include "../common.js"

// Kitten
function Kitten(email, name) {
  this.email = email;
  this.name = name;
}
Kitten.prototype.username = function() {
  var email = this.email;
  var ix = email.indexOf('@');
  return ix < 0 ? email : email.substring(0, ix);
}

// Model
function Model() {
  this.kittens = [];
  this._index = {};
}
Model.prototype.add = function(kitten) {
  this.kittens.push(kitten);
  this._index[kitten.email] = {
    callbacks: [],
    kitten: kitten
  };
}

function viewOn(element, model, kitten) {
  var text = document.create('div');
  kitten.name.split(' ').forEach(function(x) {
    text.add(document.create('div').text(x));
  });
  text.className = 'text';

  var badge = document.create('div').text('??');
  badge.className = 'badge';

  element.css('background-image', 'url(img/' + kitten.username() + '.png)')
    .text('')
    .add(text, badge);
}

function main() {
  var model = new Model();
  document.body.qa('.kitten').forEach(function(element) {
    var kitten = new Kitten(element.getAttribute('email'), element.textContent);
    model.add(kitten);
    viewOn(element, model, kitten);
  });
}

whenReady(main);
})();
