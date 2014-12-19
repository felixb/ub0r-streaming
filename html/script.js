var config = {};

var defaultServer = {'Host': 'off', 'Port': 0};
var defaultRadio = {'Uri': 'off', 'Name': 'off'};

var deleteEditId = null;
var deleteRadioId = null;

// get active radio for a given server
// result is never undefined
function getActiveRadio(server) {
    try {
        var r = config.Servers[server.Host];
        return r ? r : defaultRadio;
    } catch (err) {
        console.log(err);
        return defaultRadio;
    }
}

// get active server for a given receiver
// result is never undefined
function getActiveServer(receiver) {
    try {
        var s =  config.Receivers[receiver.Host];
        return s ? s : defaultServer;
    } catch (err) {
        console.log(err);
        return defaultServer;
    }
}

// create server html id
function getServerId(server) {
    return 'server-' + server.Host + '-' + server.Port;
}

// create receiver html id
function getReceiverId(receiver) {
    return 'receiver-' + receiver.Host;
}

// create receiver html id
function getRadioId(radio) {
    var hash = CryptoJS.SHA1(radio.Uri);
    return 'radio-' + hash.toString(CryptoJS.enc.Hex);
}

// get data-icon value
function getIcon(active, uri) {
    if (active) {
        return uri == 'off' ? 'power' : 'audio';
    } else {
        return 'false';
    }
}

// create list of radios for a single server
function injectServer(s) {
    var id = getServerId(s);
    var activeRadio = getActiveRadio(s);
    var radios = '<ul class="server-list-ul" data-role="listview" data-inset="true">';
    $.each(config.Backends.Radios, function(i, r) {
        radios += '<li data-icon="' + getIcon(r.Uri == activeRadio.Uri, r.Uri) + '"><a class="api-call" href="/api/server/' + s.Host + '/radio/' + i + '">' + r.Name + '</a></li>';
    });
    radios += '</ul>';
    var item = '<div id="' + id + '"><h4>' + s.Name + '</h4>' + radios + '</div>'
    $('#server-list').append(item);
}

// create list of servers for a single receiver
function injectReceiver(r) {
    var id = getReceiverId(r);
    var servers = '<ul class="receiver-list-ul" data-role="listview" data-inset="true">';
    var activeServer = getActiveServer(r);
    // inject 'off' server
    servers += '<li data-icon="' + getIcon('off' == activeServer.Host, 'off') + '"><a class="api-call" href="/api/receiver/' + r.Host + '/off/0">off</a></li>';
    // add servers
    if (config.Backends.Servers) {
        $.each(config.Backends.Servers, function(i, e) {
            servers += '<li data-icon="' + getIcon(e.Host == activeServer.Host, e.Host) + '"><a class="api-call" href="/api/receiver/' + r.Host + '/server/' + i + '">' + e.Name + '</a></li>';
        });
    }
    // add static servers
    if (config.Backends.StaticServers) {
        $.each(config.Backends.StaticServers, function(i, e) {
            servers += '<li data-icon="' + getIcon(e.Host == activeServer.Host, e.Host) + '"><a class="api-call" href="/api/receiver/' + r.Host + '/static/' + i + '">' + e.Name + '</a></li>';
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
    // create list of servers
    $('#server-list').empty();
    if (config.Backends.Servers && config.Backends.Servers.length > 0) {
        $.each(config.Backends.Servers, function(i, e) {
            injectServer(e);
        });
    } else {
      $('#server-list').append("no active server found");
    }

    // create list of receivers
    $('#receiver-list').empty();
    if (config.Backends.Receivers && config.Backends.Receivers.length > 0) {
        $.each(config.Backends.Receivers, function(i, e) {
            injectReceiver(e);
        });
    } else {
      $('#receiver-list').append("no active receiver found");
    }

    // create list of radios
    $('#radios-list').empty();
    if (config.Backends.Radios && config.Backends.Radios.length > 0) {
        $.each(config.Backends.Radios, function(i, e) {
            injectRadio(e);
        });
    } else {
      $('#radio-list').append("no radio defined");
    }

    // init list views to make them look beautiful
    $('.server-list-ul').listview().listview('refresh');
    $('.receiver-list-ul').listview().listview('refresh');
    $('.radio-list-ul').listview().listview('refresh');

    // replace api-calls with ajax requests
    $('.api-call').click(function(e) {
       e.preventDefault();
       $.get(e.target.href);
    });
    $('.dialog-edit-radio').click(function(e) {
       e.preventDefault();
       showEditRadioDialog(e.target.rel);
    });
    $('.dialog-delete-radio').click(function(e) {
       e.preventDefault();
       showDeleteRadioDialog(e.target.rel);
    });
    $('.dialog-add-radio').click(function(e) {
        e.preventDefault();
        showAddRadioDialog();
    });
    $('form#add-radio-form').submit(function(e) {
        addRadio();
        return false;
    });
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

function showAddRadioDialog(id) {
    editRadioId = null;
    $("#add-radio-name").val("");
    $("#add-radio-uri").val("");
    $.mobile.changePage('#add-radio');
    setTimeout(function(){
        $('#add-radio-name').focus();
    },200);
}

function showEditRadioDialog(id) {
    editRadioId = id;
    $("#add-radio-name").val(config.Backends.RadiosByKey[id].Name);
    $("#add-radio-uri").val(config.Backends.RadiosByKey[id].Uri);
    $.mobile.changePage('#add-radio');
    setTimeout(function(){
        $('#add-radio-name').focus();
    },200);
}

function showDeleteRadioDialog(id) {
    deleteRadioId = id;
    $.mobile.changePage('#delete-radio');
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
});
