import {template} from "./template.js";

let config = {};
let state = {};
let stat = {
    IncomingBytes: 0,
    OutgoingBytes: 0,
    IncomingFrames: 0,
    OutgoingFrames: 0,
    IncomingRateBytes: 0,
    OutgoingRateBytes: 0,
    IncomingRateFrames: 0,
    OutgoingRateFrames: 0,
};

$(document).ready(function () {
    $(window).on('resize', function () {
        $("#container").css("height", "calc(100% - 15pt - " + $("#top-navbar").height() + "px)");
    }).trigger('resize');

    let includes = $("[data-include]");
    $.each(includes, function () {
        $(this).load($(this).data("include"), function () {
            switch ($(this).attr("data-include")) {
                case "connection.edit.html":
                    $("#connection-submit").click(function () {
                        updateConnection();
                    });
                    $("#connection-delete").click(function () {
                        deleteConnection();
                    });
                    break;
                case "journal.html":
                    initJournal();
                    break;
            }
        });
    });

    document.addEventListener("astilectron-ready", function () {
        astilectron.onMessage(function (message) {
            console.log("server message: " + JSON.stringify(message, null, 1));
            switch (message.name) {
                case "initialization":
                    if (message.payload.Config !== null) {
                        config = message.payload.Config;
                    }
                    if (message.payload.State !== null) {
                        state = message.payload.State;
                    }
                    initialization();
                    break;
                case "statistic":
                    statistic(message.payload);
                    break;
                case "logger":
                    logger(message.payload);
                    break;
                default:
                    console.log("unknown message");
            }
        });
    });

    $("#top-navbar .nav-link").click(function () {
        showTab($(this).attr("data-bs-target"));
    });

    $("#connection-import").click(function () {
        importConnection();
    });
});

function showTab(id) {
    $("#container .tab-pane").hide();
    $(id).show();
    if (id === "#journal-tab") {
        loadJournal();
    }
}

function initialization() {
    hideSpinner();
    $("#connections").html("");
    config.Connections.forEach((conn, i) => {
        let tmpl = $(template["connection-row"]);
        tmpl.attr("id", "connection-row-" + i);
        $(tmpl.find(".label-name")[0]).html(conn.Name);
        $(tmpl.find(".addr")[0]).html(conn.Server.Host);
        $(tmpl.find(".user")[0]).html(conn.User.Name);
        $(tmpl.find(".conn-checkbox")[0]).attr("id", "conn-checkbox-" + i).attr("data-conn-id", i).attr("checked", state.Connections[i].IsConnected);
        $(tmpl.find(".conn-label")[0]).attr("for", "connection-row-" + i);
        $(tmpl.find(".conn-info")[0]).css("display", state.Connections[i].IsConnected ? "block" : "none");
        $(tmpl.find(".conn-edit")[0]).attr("data-connection-id", i);
        $("#connections").append(tmpl);
    });

    $("#connections .conn-edit").click(function () {
        editConnection($(this).attr("data-connection-id"));
    });

    $(".conn-checkbox").on("change", function () {
        commandCreateConnection($(this).attr("data-conn-id"), $(this).is(':checked'));
    });
}

function importConnection() {
    const {dialog} = require("electron").remote;
    let files = dialog.showOpenDialogSync({
        properties: ["openFile", "showHiddenFiles"],
        filters: [
            {name: 'Tapir Files', extensions: ['tapir']},
            {name: 'All Files', extensions: ['*']}
        ]
    });
    if (files === undefined) {
        return;
    }

    astilectron.sendMessage(
        {
            name: "connection.import",
            payload: {
                file: files[0]
            }
        },
        function (message) {
        }
    );
}

function commandCreateConnection(connID, connect) {
    $(".conn-checkbox").attr("disabled", true);
    showSpinner(connect ? "Connecting..." : "Disconnecting...");
    astilectron.sendMessage(
        {
            name: "connection.connect",
            payload: {
                id: parseInt(connID),
                connect: connect,
            }
        },
        function (message) {
            hideSpinner();
            $(".conn-checkbox").attr("disabled", false);
            if (message.payload.error !== "") {
                $("#conn-checkbox-" + connID).prop("checked", !connect)
                showError(message.payload.error);
                return
            }
            message.payload.data.Connections.forEach((conn, i) => {
                $("#conn-checkbox-" + conn.ID).prop("checked", conn.IsConnected);
                $("#connection-row-" + conn.ID).find(".conn-info").css("display", conn.IsConnected ? "block" : "none");
            });
        }
    );
}

function statistic(data) {
    stat = {
        IncomingBytes: 0,
        OutgoingBytes: 0,
        IncomingFrames: 0,
        OutgoingFrames: 0,
        IncomingRateBytes: 0,
        OutgoingRateBytes: 0,
        IncomingRateFrames: 0,
        OutgoingRateFrames: 0,
    };

    for (const [connID, s] of Object.entries(data)) {
        stat.IncomingBytes += s.IncomingBytes;
        stat.OutgoingBytes += s.OutgoingBytes;
        stat.IncomingFrames += s.IncomingFrames;
        stat.OutgoingFrames += s.OutgoingFrames;
        stat.IncomingRateBytes += s.IncomingRateBytes;
        stat.OutgoingRateBytes += s.OutgoingRateBytes;
        stat.IncomingRateFrames += s.IncomingRateFrames;
        stat.OutgoingRateFrames += s.OutgoingRateFrames;

        let conn = $("#connection-row-" + connID);
        conn.find(".down-speed").html(formatSpeed(s.IncomingRateBytes));
        conn.find(".up-speed").html(formatSpeed(s.OutgoingRateBytes));
    }

    let s = $("#statistics");
    s.find(".bytes-in").html(formatBytes(stat.IncomingBytes));
    s.find(".bytes-out").html(formatBytes(stat.OutgoingBytes));
    s.find(".packets-in").html(formatNumber(stat.IncomingFrames));
    s.find(".packets-out").html(formatNumber(stat.OutgoingFrames));
    s.find(".bytes-rate-in").html(formatSpeed(stat.IncomingRateBytes));
    s.find(".bytes-rate-out").html(formatSpeed(stat.OutgoingRateBytes));
    s.find(".packets-rate-in").html(stat.IncomingRateFrames + " pps");
    s.find(".packets-rate-out").html(stat.OutgoingRateFrames + " pps");
}

function loadJournal() {
    showSpinner("Loading...");
    astilectron.sendMessage({name: "logs.get"},
        function (message) {
            showLog(message.payload.data);
            hideSpinner();
        }
    );
}

function showLog(data) {
    let journal = $("#journal");
    journal.html("");

    Array(data)[0].forEach((line) => {
        let item = JSON.parse(line);
        let tmpl = $(template["journal-row"]);
        if (item.level === "warn" || item.level === "error" || item.level === "fatal") {
            tmpl.addClass("error");
        }

        let msg = item.message;
        for (let [key, value] of Object.entries(item)) {
            if (key === "time" || key === "level" || key === "message") {
                continue
            }
            msg += "&nbsp;<b>" + key + "</b>=" + value;
        }

        $(tmpl.find(".time")[0]).html(item.time);
        $(tmpl.find(".message")[0]).html(msg);
        journal.append(tmpl);
    });
}

function editConnection(connID) {
    $("#connection-name").val(config.Connections[connID].Name);
    $("#server-host").val(config.Connections[connID].Server.Host);
    $("#port-from").val(config.Connections[connID].Server.PortMin);
    $("#port-to").val(config.Connections[connID].Server.PortMax);
    let proto = "udp";
    if (config.Connections[connID].Server.UseTCP && config.Connections[connID].Server.UseUDP) {
        proto = "both";
    } else if (config.Connections[connID].Server.UseTCP) {
        proto = "tcp";
    }
    $("#proto option[value=\"" + proto + "\"]").attr("selected", "selected");
    $("#username").val(config.Connections[connID].User.Name);
    $("#password").val(config.Connections[connID].User.Password);
    $("#connection-edit-id").val(connID);
    showTab("#connection-edit-tab");
}

function updateConnection() {
    let proto = $("#proto option:selected").val();
    astilectron.sendMessage(
        {
            name: "connection.update",
            payload: {
                id: parseInt($("#connection-edit-id").val()),
                name: $("#connection-name").val(),
                serverHost: $("#server-host").val(),
                portMin: parseInt($("#port-from").val()),
                portMax: parseInt($("#port-to").val()),
                useTCP: proto === "both" || proto === "tcp",
                useUDP: proto === "both" || proto === "udp",
                username: $("#username").val(),
                password: $("#password").val()
            }
        },
        function (message) {
            showTab("#connections-tab");
        }
    );
}

function deleteConnection() {
    bootbox.confirm({
        title: "Deleting a connection",
        message: "Are you sure you want to delete this connection?",
        centerVertical: true,
        size: "small",
        buttons: {
            confirm: {
                label: "YES",
                className: "btn btn-danger"
            },
            cancel: {
                label: "NO",
                className: "btn btn-primary"
            }
        },
        callback: function (result) {
            if (result) {
                astilectron.sendMessage(
                    {
                        name: "connection.delete",
                        payload: {
                            id: parseInt($("#connection-edit-id").val()),
                        }
                    },
                    function (message) {
                        showTab("#connections-tab");
                    }
                );
            }
        }
    });
}

function initJournal() {
    $("#journal-scroll-down").click(function () {
        document.getElementById("journal-bottom").scrollIntoView();
    });
}

function showSpinner(title) {
    $("#spinner .title").html(title);
    $("#spinner").show();
}

function hideSpinner() {
    $("#spinner").hide();
}

function showError(body) {
    let toast = $("#toast-error");
    toast.find(".toast-body").html(body);
    toast.toast('show');
}

function formatSpeed(v) {
    if (v < 1000) {
        return v + " Bps";
    } else if (v / 1000 < 1000) {
        return (v / 1000).toFixed(2) + " KBps";
    } else if (v / 1000000 < 1000) {
        return (v / 1000000).toFixed(2) + " MBps";
    } else {
        return (v / 1000000000).toFixed(2) + " GBps";
    }
}

function formatBytes(v) {
    if (v < 1000) {
        return v + " B";
    } else if (v / 1000 < 1000) {
        return (v / 1000).toFixed(2) + " KB";
    } else if (v / 1000000 < 1000) {
        return (v / 1000000).toFixed(2) + " MB";
    } else {
        return (v / 1000000000).toFixed(2) + " GB";
    }
}

function formatNumber(v) {
    if (v < 1000) {
        return v;
    } else if (v / 1000 < 1000) {
        return (v / 1000).toFixed(2) + "K";
    } else if (v / 1000000 < 1000) {
        return (v / 1000000).toFixed(2) + "M";
    } else {
        return (v / 1000000000).toFixed(2) + "B";
    }
}

