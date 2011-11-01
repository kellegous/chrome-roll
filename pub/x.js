(function(){

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

function domDidLoad() {
  xhrGet('/chrome',
    function(xhr) {
      console.log(JSON.parse(xhr.responseText));
    },
    function(xhr) {
    });
  console.log('loaded');
}

addEventListener('DOMContentLoaded', domDidLoad, false);
})()
