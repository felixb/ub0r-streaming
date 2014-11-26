var backends = {};
var config = {};

function getActiveRadio(server) {
    radioId = -1;
    try {
        radio = config.Servers[server.Host];
        radioUri = radio ? radio.Uri : 'off';
        $.each(backends.Radios, function(i, r) {
            if (r.Uri == radioUri) {
                radioId = i;
            }
        });
    } catch (err) {
        console.log(err);
    }
    return radioId;
}

function getActiveServer(receiver) {
    serverId = 0;
    try {
        server = config.Receivers[receiver];
        // 0 == off
        $.each(backends.Servers, function(i, s) {
            if (s.Host == server.Host && s.Port == server.Port) {
                serverId = i + 1;
            }
        });
        $.each(backends.StaticServers, function(i, s) {
            if (s.Host == server.Host &&  s.Port == server.Port) {
                serverId = i + backends.Servers.length + 1;
            }
        });
    } catch (err) {
        console.log(err);
    }
    return serverId;
}

function getServerId(server) {
    return 'server-' + server.Host + '-' + server.Port;
}

function getReceiverId(receiver) {
    return 'receiver-' + receiver;
}

function getRadioId(serverId, radioId) {
    return 'radio-' + serverId + '-' + radioId;
}

function getReceiverServerId(receiver, serverId) {
    return 'server-' + receiver + '-' + serverId;
}

function updateConfig(data) {
    config = data;
    if ($.isEmptyObject(backends)) {
        $.get('/api/backends', injectBackends);
    } else {
        injectBackends(backends);
    }
}

function fetchConfig() {
    $.get('/api/config', updateConfig);
}

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

// callback to create html elements representing the backends
function injectBackends(data) {
    backends = data;

    $('#server-list').empty();
    $.each(backends.Servers, function(i, e) {
        var id = getServerId(e);
        var activeRadioNum = getActiveRadio(e);
        var radios = '<ul class="server-list-ul" data-role="listview" data-inset="true">';
        $.each(backends.Radios, function(i, r) {
            var dataIcon = i == activeRadioNum ? 'audio' : 'false';
            radios += '<li data-icon="' + dataIcon + '"><a class="api-call" href="/api/server/' + e.Host + '/radio/' + i + '">' + r.Name + '</a></li>';
        });
        radios += '</ul>';
        var item = '<div id="' + id + '"><h4>' + e.Host + '</h4>' + radios + '</div>'
        $('#server-list').append(item);
    });

    $('#receiver-list').empty();
    $.each(backends.Receivers, function(i, k) {
        var id = getReceiverId(k);
        var servers = '<ul class="receiver-list-ul" data-role="listview" data-inset="true">';
        var activeServerNum = getActiveServer(k);
        console.log(k + " | active server num: " + activeServerNum);
        var dataIcon = 0 == activeServerNum ? 'audio' : 'false';
        servers += '<li data-icon="' + dataIcon + '"><a class="api-call" href="/api/receiver/' + k + '/off/0">off</a></li>';
        if (backends.Servers) {
            var offset = 1;
            $.each(backends.Servers, function(i, e) {
                var serverNum = i + offset;
                var dataIcon = serverNum == activeServerNum ? 'audio' : 'false';
                servers += '<li data-icon="' + dataIcon + '"><a class="api-call" href="/api/receiver/' + k + '/server/' + i + '">' + e.Host + '</a></li>';
            });
        }
        if (backends.StaticServers) {
            var offset = 1 + backends.Servers.length;
            $.each(backends.StaticServers, function(i, e) {
                var serverNum = i + offset;
                var dataIcon = serverNum == activeServerNum ? 'audio' : 'false';
                servers += '<li data-icon="' + dataIcon + '"><a class="api-call" href="/api/receiver/' + k + '/static/' + i + '">' + e.Host + '</a></li>';
            });
        }
        servers += '</ul>';
        $('#receiver-list').append('<div id="' + id + '"><h4>' + k + '</h4>' + servers + '</div>');
    });

    $('.server-list-ul').listview().listview('refresh');
    $('.receiver-list-ul').listview().listview('refresh');

    $('.api-call').click(function(e) {
       e.preventDefault();
       $.get(e.target.href);
       // TODO $.get('/config', updateConfig);
       // TODO mark button active: ui-btn-active
    });
}

$(document).ready(function() {
    fetchConfig();
    watchConfig();
});
