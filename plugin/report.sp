#include <sourcemod>
#include <morecolors>

public Plugin myinfo = {
    name = "Report System",
    author = "R4G3_BABY",
    description = "Report players to admins",
    version = "1.0",
    url = "https://r4g3baby.com"
};

int targets[MAXPLAYERS + 1];
float lastUsed[MAXPLAYERS + 1];
float lastReported[MAXPLAYERS + 1];

ConVar cvarConfig;
ConVar cvarUseDelay;
ConVar cvarReportedDelay;
ConVar cvarAnnounceMsg;
ConVar cvarAnnounceDelay;

Database hDatabase = null;
public void SetDatabase(Database db, const char[] error, any data) {
    if (db == null) {
        LogError("Database failure: %s", error);
    } else hDatabase = db;
}

public void OnPluginStart() {
    cvarConfig = CreateConVar("sm_report_config", "default", "Specifies the config server name for the bot to use.")
    cvarUseDelay = CreateConVar("sm_report_use_delay", "60.0", "Time, in seconds, to prevent players from reporting again.", _, true, 0.0);
    cvarReportedDelay = CreateConVar("sm_report_reported_delay", "180.0", "Time, in seconds, to prevent players from being reported again.", _, true, 0.0);
    cvarAnnounceMsg = CreateConVar("sm_report_announce_msg", "{#a020f0}[{#00ffff}Report{#a020f0}] {#00ffff}See a player breaking the rules? - Type {#ffff00}!report {#00ffff}in chat.", "Message to display when announcing the report system.")
    cvarAnnounceDelay = CreateConVar("sm_report_announce_delay", "480.0", "Time, in seconds, to display the announce message.", _, true, 0.0);

    Database.Connect(SetDatabase, SQL_CheckConfig("reports") ? "reports" : "default");

    RegConsoleCmd("sm_report", Command_Report, "report a player");

    LoadTranslations("common.phrases.txt");

    if (GetConVarFloat(cvarAnnounceDelay) > 0) {
        CreateTimer(GetConVarFloat(cvarAnnounceDelay), Announce_Report, _, TIMER_REPEAT);
    }
}

public void OnClientPutInServer(client) {
    lastUsed[client] = 0.0;
    lastReported[client] = 0.0;
}

public Action Announce_Report(Handle timer) {
    char announceMsg[255]; GetConVarString(cvarAnnounceMsg, announceMsg, sizeof(announceMsg));
    MC_PrintToChatAll(announceMsg);
    return Plugin_Continue;
}

public Action Command_Report(int client, int args) {
    if (client == 0) {
        ReplyToCommand(client, "[SM] Console can't report players.");
        return Plugin_Handled;
    }

    if (lastUsed[client] != 0.0 && lastUsed[client] + GetConVarFloat(cvarUseDelay) > GetGameTime()) {
        PrintToChat(client, "[SM] You must wait %i seconds before submitting another report.", RoundFloat((lastUsed[client] + RoundFloat(GetConVarFloat(cvarUseDelay))) - RoundFloat(GetGameTime())));
        return Plugin_Handled;
    }

    char target[MAX_NAME_LENGTH], reason[255];

    if (args >= 1) {
        GetCmdArg(1, target, sizeof(target));
    } else {
        OpenTargetsMenu(client);
        return Plugin_Handled;
    }

    int targetC = FindTarget(client, target, true, false);
    if (targetC == -1) {
        return Plugin_Handled;
    }

    if (args >= 2) {
        GetCmdArg(2, reason, sizeof(reason));
        for (int i = 3; i < args+1; i++){
            char tmp[32];
            GetCmdArg(i, tmp, sizeof(tmp));
            StrCat(reason, 255, " ");
            StrCat(reason, 255, tmp);
        }
    } else {
        OpenReasonsMenu(client, targetC);
        return Plugin_Handled;
    }

    ReportPlayer(client, targetC, reason);

    return Plugin_Handled;
}

void ReportPlayer(int client, int target, const char[] reason) {
    if (client == target) {
        PrintToChat(client, "[SM] Why would you report yourself?")
        return;
    }

    if (lastReported[target] != 0.0 && lastReported[target] + GetConVarFloat(cvarReportedDelay) > GetGameTime()) {
        PrintToChat(client, "[SM] A report for this player has already been issued.");
        return;
    }

    if (hDatabase != null) {
        char clientSteamID[32]; GetClientAuthId(client, AuthId_SteamID64, clientSteamID, sizeof(clientSteamID));
        char clientSteamV2ID[32]; GetClientAuthId(client, AuthId_Steam2, clientSteamV2ID, sizeof(clientSteamV2ID));
        char targetSteamID[32]; GetClientAuthId(target, AuthId_SteamID64, targetSteamID, sizeof(targetSteamID));
        char targetSteamV2ID[32]; GetClientAuthId(target, AuthId_Steam2, targetSteamV2ID, sizeof(targetSteamV2ID));

        char config[255]; GetConVarString(cvarConfig, config, sizeof(config));

        DataPack dataPack = new DataPack();
        dataPack.WriteCell(client);
        dataPack.WriteString(clientSteamV2ID);
        dataPack.WriteCell(target);
        dataPack.WriteString(targetSteamV2ID);
        dataPack.WriteString(reason);
        dataPack.Reset();

        int escapedConfigLength = strlen(config) * 2 + 1
        char[] escapedConfig = new char[escapedConfigLength];
        SQL_EscapeString(hDatabase, config, escapedConfig, escapedConfigLength);

        int escapedClientSteamIDLength = strlen(clientSteamID) * 2 + 1
        char[] escapedClientSteamID = new char[escapedClientSteamIDLength];
        SQL_EscapeString(hDatabase, clientSteamID, escapedClientSteamID, escapedClientSteamIDLength);

        int escapedTargetSteamIDLength = strlen(targetSteamID) * 2 + 1
        char[] escapedTargetSteamID = new char[escapedTargetSteamIDLength];
        SQL_EscapeString(hDatabase, targetSteamID, escapedTargetSteamID, escapedTargetSteamIDLength);

        int escapedReasonLength = strlen(reason) * 2 + 1
        char[] escapedReason = new char[escapedReasonLength];
        SQL_EscapeString(hDatabase, reason, escapedReason, escapedReasonLength);

        int escapedHostIPLength = strlen(hostIP) * 2 + 1
        char[] escapedHostIP = new char[escapedHostIPLength];
        SQL_EscapeString(hDatabase, hostIP, escapedHostIP, escapedHostIPLength);

        char query[512]; Format(query, sizeof(query), "INSERT INTO reports(config, clientSteamID, targetSteamID, reason, hostip) VALUES ('%s', '%s', '%s', '%s', '%s')", escapedConfig, escapedClientSteamID, escapedTargetSteamID, escapedReason, escapedHostIP)
        hDatabase.Query(PostDBQuery, query, dataPack);

        lastUsed[client] = GetGameTime();
        lastReported[target] = GetGameTime();
    } else {
        PrintToChat(client, "[SM] Failed to submit report, please try again.");

        // Database might have been down at startup so we try to get a new connection again
        Database.Connect(SetDatabase, SQL_CheckConfig("reports") ? "reports" : "default");
    }
}

public void PostDBQuery(Database db, DBResultSet results, const char[] error, any data) {
    DataPack dataPack = view_as<DataPack>(data);
    int client = dataPack.ReadCell();
    char clientSteamV2ID[32]; dataPack.ReadString(clientSteamV2ID, sizeof(clientSteamV2ID));
    int target = dataPack.ReadCell();
    char targetSteamV2ID[32]; dataPack.ReadString(targetSteamV2ID, sizeof(targetSteamV2ID));
    char reason[255]; dataPack.ReadString(reason, sizeof(reason));
    CloseHandle(dataPack);

    if (db == null || results == null || error[0] != '\0') {
        LogError("Query failed: %s", error);
        PrintToChat(client, "[SM] Failed to submit report, please try again.");
        return;
    }

    PrintToChat(client, "[SM] Reported player %N[%s] for %s.", target, targetSteamV2ID, reason);
    LogMessage("%N[%s] reported player %N[%s] for %s.", client, clientSteamV2ID, target, targetSteamV2ID, reason);

    for (int i = 1; i <= MaxClients; i++) {
        if (IsClientConnected(i) && client != i && CheckCommandAccess(i, "sm_admin", ADMFLAG_GENERIC)) {
            PrintToChat(i, "[SM] %N[%s] reported player %N[%s] for %s.", client, clientSteamV2ID, target, targetSteamV2ID, reason)
        }
    }
}

public int ChooseTargetHandler(Menu menu, MenuAction action, int param1, int param2) {
    if (action == MenuAction_Select) {
        char client[12]; menu.GetItem(param2, client, sizeof(client));
        OpenReasonsMenu(param1, StringToInt(client));
    } else if (action == MenuAction_End) {
        delete menu;
    }
    return 0
}

public int ChooseReasonHandler(Menu menu, MenuAction action, int param1, int param2) {
    if (action == MenuAction_Select) {
        char reason[32]; menu.GetItem(param2, reason, sizeof(reason));
        ReportPlayer(param1, targets[param1], reason);
    } else if (action == MenuAction_Cancel && param2 == MenuCancel_ExitBack) {
        OpenTargetsMenu(param1)
    } else if (action == MenuAction_End) {
        delete menu;
    }
    return 0
}

void OpenTargetsMenu(int client) {
    Menu menu = new Menu(ChooseTargetHandler, MENU_ACTIONS_ALL);
    menu.SetTitle("Report Player");
    menu.ExitBackButton = false;
    AddTargetsToMenu(menu);
    menu.Display(client, MENU_TIME_FOREVER);
}

void OpenReasonsMenu(int client, int target) {
    targets[client] = target;

    Menu menu = new Menu(ChooseReasonHandler, MENU_ACTIONS_ALL);
    menu.SetTitle("Choose Reason");
    menu.ExitBackButton = true;
    AddReasonsToMenu(menu);
    menu.Display(client, MENU_TIME_FOREVER);
}

void AddTargetsToMenu(Menu menu) {
    char client[12];
    char name[MAX_NAME_LENGTH];

    for (int i = 1; i <= MaxClients; i++) {
        if (!IsClientConnected(i) || IsClientInKickQueue(i) || IsFakeClient(i)) {
            continue;
        }

        IntToString(i, client, sizeof(client));
        GetClientName(i, name, sizeof(name));

        menu.AddItem(client, name);
    }
}

void AddReasonsToMenu(Menu menu) {
    menu.AddItem("Hacking", "Hacking");
    menu.AddItem("Chat/Mic Spam", "Chat/Mic Spam");
    menu.AddItem("Advertising Links", "Advertising Links")
    menu.AddItem("Admin Impersonation", "Admin Impersonation")
    menu.AddItem("Porn/Gore Sprays", "Porn/Gore Sprays")
    menu.AddItem("Exploiting", "Exploiting")
    // menu.AddItem("Other", "Other") allow user to specify reason
}