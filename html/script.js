var backends = {};
var config = {};

var defaultServer = {'Host': 'off', 'Port': 0};
var defaultRadio = {'Uri': 'off', 'Name': 'off'};

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
        var s =  config.Receivers[receiver];
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
    return 'receiver-' + receiver;
}

// translate server/receiver into name
// uses backends.Name map
function getName(e) {
    name = backends.Names[e];
    return name !== 'undefined' ? name : e;
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
    $.each(backends.Radios, function(i, r) {
        radios += '<li data-icon="' + getIcon(r.Uri == activeRadio.Uri, r.Uri) + '"><a class="api-call" href="/api/server/' + s.Host + '/radio/' + i + '">' + r.Name + '</a></li>';
    });
    radios += '</ul>';
    var item = '<div id="' + id + '"><h4>' + getName(s.Host) + '</h4>' + radios + '</div>'
    $('#server-list').append(item);
}

// create list of servers for a single receiver
function injectReceiver(r) {
    var id = getReceiverId(r);
    var servers = '<ul class="receiver-list-ul" data-role="listview" data-inset="true">';
    var activeServer = getActiveServer(r);
    // inject 'off' server
    servers += '<li data-icon="' + getIcon('off' == activeServer.Host, 'off') + '"><a class="api-call" href="/api/receiver/' + r + '/off/0">' + getName('off') + '</a></li>';
    // add servers
    if (backends.Servers) {
        $.each(backends.Servers, function(i, e) {
            servers += '<li data-icon="' + getIcon(e.Host == activeServer.Host, e.Host) + '"><a class="api-call" href="/api/receiver/' + r + '/server/' + i + '">' + getName(e.Host) + '</a></li>';
        });
    }
    // add static servers
    if (backends.StaticServers) {
        $.each(backends.StaticServers, function(i, e) {
            servers += '<li data-icon="' + getIcon(e.Host == activeServer.Host, e.Host) + '"><a class="api-call" href="/api/receiver/' + r + '/static/' + i + '">' + getName(e.Host) + '</a></li>';
        });
    }
    servers += '</ul>';
    $('#receiver-list').append('<div id="' + id + '"><h4>' + getName(r) + '</h4>' + servers + '</div>');
}

// callback to create html elements representing the backends
function injectBackends(data) {
    backends = data;

    // create list of servers
    $('#server-list').empty();
    $.each(backends.Servers, function(i, e) {
        injectServer(e);
    });

    // create list of receivers
    $('#receiver-list').empty();
    $.each(backends.Receivers, function(i, e) {
        injectReceiver(e);
    });

    // init list views to make them look beautiful
    $('.server-list-ul').listview().listview('refresh');
    $('.receiver-list-ul').listview().listview('refresh');

    // replace api-calls with ajax requests
    $('.api-call').click(function(e) {
       e.preventDefault();
       $.get(e.target.href);
    });
}

// callback to update config and fetch backends still undef
function updateConfig(data) {
    config = data;
    if ($.isEmptyObject(backends)) {
        $.get('/api/backends', injectBackends);
    } else {
        injectBackends(backends);
    }
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

// init page
$(document).ready(function() {
    fetchConfig();
    watchConfig();
});
