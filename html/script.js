var config = {};

var defaultServer = {'Host': 'off', 'Port': 0};
var defaultRadio = {'Uri': 'off', 'Name': 'off'};
var offId = getServerId(defaultServer);

var deleteEditId = null;
var deleteRadioId = null;

function isNotEmpty(o) {
    return o && Object.keys(o).length > 0
}

function sortNames(a, b) {
    var an = a.Name.toLowerCase();
    var bn = b.Name.toLowerCase();
    if (an < bn) return -1;
    if (an > bn) return 1;
    return 0
}

function eachSorted(obj, s, f) {
    var keys = Object.keys(obj);
    keys.sort(function(a, b){return s(obj[a], obj[b])});
    $.each(keys, function(i, k) {f(k, obj[k])})
}

// get active radio for a given server
// result is never undefined
function getActiveRadio(server) {
    try {
        var id = config.Servers[getServerId(server)]
        var r = id ? config.Backends.Radios[id] : null;
        return  r ? r : defaultRadio;
    } catch (err) {
        console.log(err);
        return defaultRadio;
    }
}

// get active server for a given receiver
// result is never undefined
function getActiveServer(receiver) {
    // TODO handle internal servers -> radioId?
    try {
        var id = config.Receivers[getReceiverId(receiver)];
        var s = id ? config.Backends.Servers[id] : null;
        return s ? s : defaultServer;
    } catch (err) {
        console.log(err);
        return defaultServer;
    }
}

// create server html id
function getServerId(server) {
    return 'server-' + server.Host + ':' + server.Port;
}

// create receiver html id
function getReceiverId(receiver) {
    return 'receiver-' + receiver.Name;
}

// create receiver html id
function getRadioId(radio) {
    var hash = CryptoJS.SHA1(radio.Uri);
    return 'radio-' + hash.toString(CryptoJS.enc.Hex);
}

// get data-icon value
function getIcon(active, off) {
    if (active) {
        return off ? 'power' : 'audio';
    } else {
        return 'false';
    }
}

// create list of servers for a single receiver
function injectReceiver(r) {
    var id = getReceiverId(r);
    var servers = '<ul class="receiver-list-ul" data-role="listview" data-inset="true">';
    var activeServer = getActiveServer(r);
    var activeId = getServerId(activeServer);
    var activeRadio = getActiveRadio(activeServer);
    var activeRadioId = getRadioId(activeRadio);
    // inject 'off' server
    servers += '<li data-icon="' + getIcon(offId == activeId, true) + '"><a class="api-call" href="/api/receiver/?receiver=' + id + '&server=off">Off</a></li>';
    // add servers
    if (config.Backends.Servers) {
        eachSorted(config.Backends.Servers, sortNames, function(k, e) {
            if (!e.Internal) {
                servers += '<li data-icon="' + getIcon(k == activeId, false) + '"><a class="api-call" href="/api/receiver/?receiver=' + id + '&server=' + k + '">' + e.Name + '</a></li>';
            }
        });
    }
    // add radios
    if (config.Backends.Radios) {
        eachSorted(config.Backends.Radios, sortNames, function(k, e) {
            servers += '<li data-icon="' + getIcon(k == activeRadioId, false) + '"><a class="api-call" href="/api/receiver/?receiver=' + id + '&radio=' + k + '">' + e.Name + '</a></li>';
        });
    }
    servers += '</ul>';
    $('#receiver-list').append('<div id="' + id + '"><h4>' + r.Name + '</h4>' + servers + '</div>');
}

// create list radios
function injectRadio(r) {
    var id = getRadioId(r);
    radio = '<li id="' + id + '"><div class="ui-grid-a">';
    radio += '<div class="ui-block-a">';
    radio += '<h2>' + r.Name + '</h2>';
    radio += '<p>' + r.Uri + '</p>';
    radio += '</div>';
    radio += '<div class="ui-block-b" style="text-align: right;">';
    radio += '<a href="#" rel="' + id + '" class="ui-btn ui-btn-inline ui-icon-edit   ui-btn-icon-notext ui-corner-all ui-shadow dialog-edit-radio" data-icon="edit">Edit</a>';
    radio += '<a href="#" rel="' + id + '" class="ui-btn ui-btn-inline ui-icon-delete ui-btn-icon-notext ui-corner-all ui-shadow dialog-delete-radio" data-icon="delete">Delete</a>';
    radio += '</div>';
    radio += '</div></li>';
    $('#radios-list').append(radio);
}

// create html elements representing the backends
function injectBackends() {
    // create list of receivers
    $('#receiver-list').empty();
    if (isNotEmpty(config.Backends.Receivers)) {
        eachSorted(config.Backends.Receivers, sortNames, function(k, e) {
            injectReceiver(e);
        });
    } else {
      $('#receiver-list').append("no active receiver found");
    }

    // create list of radios
    $('#radios-list').empty();
    if (isNotEmpty(config.Backends.Radios)) {
        eachSorted(config.Backends.Radios, sortNames, function(k, e) {
            injectRadio(e);
        });
    } else {
      $('#radio-list').append("no radio defined");
    }

    // init list views to make them look beautiful
    $('.receiver-list-ul').listview().listview('refresh');
    $('.radio-list-ul').listview().listview('refresh');

    // replace api-calls with ajax requests
    $('.api-call').unbind('click', onApiCallClick);
    $('.api-call').click(onApiCallClick);
    $('.dialog-edit-radio').unbind('click', onEditRadioClick);
    $('.dialog-edit-radio').click(onEditRadioClick);
    $('.dialog-delete-radio').unbind('click', onDeleteRadioClick);
    $('.dialog-delete-radio').click(onDeleteRadioClick);
}

function onApiCallClick(e) {
   e.preventDefault();
   $.get(e.target.href);
}

function onAddRadioClick(e) {
    e.preventDefault();
    showEditRadioDialog(null);
}

function onEditRadioClick(e) {
   e.preventDefault();
   showEditRadioDialog(e.target.rel);
}

function onDeleteRadioClick(e) {
   e.preventDefault();
   showDeleteRadioDialog(e.target.rel);
}

function onAddRadioSubmit(e) {
    e.preventDefault();
    addRadio();
}

function onDeleteRadioSubmit(e) {
    e.preventDefault();
    deleteRadio();
}

// callback to update config
function updateConfig(data) {
    // enrich config data
    data.Backends.RadiosByKey = {};
    $.each(data.Backends.Radios, function(k,e) {
        data.Backends.RadiosByKey[getRadioId(e)] = e;
    });

    config = data;
    injectBackends();
}

// fetch config in background
function fetchConfig() {
    $.get('/api/config', updateConfig);
}

// watch for config changes with web sockets
function watchConfig() {
    if(typeof(WebSocket) === 'undefined') {
        console.log('WebSocket not supported')
        return
    }

    var wsUrl = 'ws://' + window.location.host + window.location.pathname + 'ws/config';
    var ws = new WebSocket(wsUrl);
    ws.onmessage = function(msg) {
        updateConfig($.parseJSON(msg.data));
    };
}

function showEditRadioDialog(id) {
    editRadioId = id;
    if (id) {
        $("#add-radio-name").val(config.Backends.RadiosByKey[id].Name);
        $("#add-radio-uri").val(config.Backends.RadiosByKey[id].Uri);
    } else {
        $("#add-radio-name").val("");
        $("#add-radio-uri").val("");
    }
    $('#add-radio-name').toggleClass('error', false);
    $('#add-radio-uri').toggleClass('error', false);
    $.mobile.changePage('#add-radio');
    setTimeout(function(){
        $('#add-radio-name').focus();
    },200);
}

function showDeleteRadioDialog(id) {
    deleteRadioId = id;
    $.mobile.changePage('#delete-radio');
    setTimeout(function(){
        $('#delete-radio-button').focus();
    },200);
}

function addRadio() {
    var name = $('#add-radio-name').val();
    var uri = $('#add-radio-uri').val();
    if (name.length > 0 && uri.length > 0) {
        $.ajax({url: '/api/radio?id=' + editRadioId,
            data: JSON.stringify({"Uri": uri, "Name": name}),
            type: 'post',
            async: 'true',
            dataType: 'json'});
        $.mobile.back();
    } else {
        $('#add-radio-name').toggleClass('error', name.length == 0);
        $('#add-radio-uri').toggleClass('error', uri.length == 0);
    }
}

function deleteRadio() {
    $.ajax({
        url: "/api/radio?id=" + deleteRadioId,
        type: "delete"
    });
    $.mobile.back();
}

// init page
$(document).ready(function() {
    fetchConfig();
    watchConfig();

    $('.dialog-add-radio').unbind('click', onAddRadioClick);
    $('.dialog-add-radio').click(onAddRadioClick);
    $('form#add-radio-form').unbind('submit', onAddRadioSubmit);
    $('form#add-radio-form').submit(onAddRadioSubmit);
    $('form#delete-radio-form').unbind('submit', onDeleteRadioSubmit);
    $('form#delete-radio-form').submit(onDeleteRadioSubmit);
});
