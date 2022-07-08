#include <morecolorsm>
#include <sourcemod>

public Plugin myinfo =
{
	name        = "Report System",
	author      = "R4G3_BABY",
	description = "Report players to admins",
	version     = "1.0.0",
	url         = "https://r4g3baby.com"
};

int   targets[MAXPLAYERS + 1];
char  reasons[MAXPLAYERS + 1][256];

float lastUsed[MAXPLAYERS + 1];
float lastReported[MAXPLAYERS + 1];
float lastServerReport;

ConVar cvarConfig;
ConVar cvarUseDelay;
ConVar cvarReportedDelay;
ConVar cvarAnnounceMsg;
ConVar cvarAnnounceDelay;

char serverIP[46];

Database hDatabase = null;

public void SetDatabase(Database db, const char[] error, any data)
{
	if (db == null) LogError("Database failure: %s", error);
	else hDatabase = db;
}

public void OnPluginStart()
{
	cvarConfig        = CreateConVar("sm_report_config", "default", "Specifies the config server name for the bot to use.");
	cvarUseDelay      = CreateConVar("sm_report_use_delay", "60.0", "Time, in seconds, to prevent players from reporting again.", _, true, 0.0);
	cvarReportedDelay = CreateConVar("sm_report_reported_delay", "180.0", "Time, in seconds, to prevent players from being reported again.", _, true, 0.0);
	cvarAnnounceMsg   = CreateConVar("sm_report_announce_msg", "{#a020f0}[{#00ffff}Report System{#a020f0}]{#00ffff} - Type {#ffff00}!report{#00ffff} in chat. {#ff0000}[WARNING! Abuse will get you banned!]", "Message to display when announcing the report system.");
	cvarAnnounceDelay = CreateConVar("sm_report_announce_delay", "480.0", "Time, in seconds, to display the announce message.", _, true, 0.0);

	int hostIP = GetConVarInt(FindConVar("hostip"));
	int pieces[4];
	pieces[0] = (hostIP >> 24) & 0x000000FF;
	pieces[1] = (hostIP >> 16) & 0x000000FF;
	pieces[2] = (hostIP >> 8) & 0x000000FF;
	pieces[3] = hostIP & 0x000000FF;
	Format(serverIP, sizeof(serverIP), "%d.%d.%d.%d:%d", pieces[0], pieces[1], pieces[2], pieces[3], GetConVarInt(FindConVar("hostport")));

	Database.Connect(SetDatabase, SQL_CheckConfig("reports") ? "reports" : "default");

	RegConsoleCmd("sm_report", Command_Report, "report a player");

	LoadTranslations("common.phrases.txt");

	if (GetConVarFloat(cvarAnnounceDelay) > 0)
	{
		CreateTimer(GetConVarFloat(cvarAnnounceDelay), Announce_Report, _, TIMER_REPEAT);
	}
}

public void OnClientPutInServer(int client)
{
	lastUsed[client]     = 0.0;
	lastReported[client] = 0.0;
}

public Action Announce_Report(Handle timer)
{
	char announceMsg[256]; GetConVarString(cvarAnnounceMsg, announceMsg, sizeof(announceMsg));
	MC_PrintToChatAll(announceMsg);
	return Plugin_Continue;
}

public Action Command_Report(int client, int args)
{
	if (client == 0)
	{
		ReplyToCommand(client, "[Report] Console can't report players.");
		return Plugin_Handled;
	}

	if (lastUsed[client] != 0.0 && lastUsed[client] + GetConVarFloat(cvarUseDelay) > GetGameTime())
	{
		MC_PrintToChat(client, "{#a020f0}[{#00ffff}Report{#a020f0}] {#aaaaaa}You must wait {#555555}%i seconds {#aaaaaa}before submitting another report.", RoundFloat((lastUsed[client] + RoundFloat(GetConVarFloat(cvarUseDelay))) - RoundFloat(GetGameTime())));
		return Plugin_Handled;
	}

	char targetArg[MAX_TARGET_LENGTH];
	if (args >= 1)
	{
		GetCmdArg(1, targetArg, sizeof(targetArg));
	}
	else
	{
		if (IsAdminOnline())
		{
			FakeClientCommand(client, "say /admins");
			OpenAdminOnlineMenu(client);
		}
		else OpenTargetsMenu(client);
		return Plugin_Handled;
	}

	int target = FindTarget(client, targetArg, true, false);
	if (target == -1)
	{
		MC_PrintToChat(client, "{#a020f0}[{#00ffff}Report{#a020f0}] {#aaaaaa}Could not find a valid target.");
		return Plugin_Handled;
	}

	targets[client] = target;
	OpenReasonsMenu(client, false);

	return Plugin_Handled;
}

public void ReportPlayer(int client, int target, const char[] reason)
{
	if (client == target)
	{
		MC_PrintToChat(client, "{#a020f0}[{#00ffff}Report{#a020f0}] {#ff5555}Why would you report yourself?");
		return;
	}

	if (lastReported[target] != 0.0 && lastReported[target] + GetConVarFloat(cvarReportedDelay) > GetGameTime())
	{
		MC_PrintToChat(client, "{#a020f0}[{#00ffff}Report{#a020f0}] {#aaaaaa}A report for this player has already been issued.");
		return;
	}

	if (hDatabase != null)
	{
		char clientSteamID[32]; GetClientAuthId(client, AuthId_SteamID64, clientSteamID, sizeof(clientSteamID));
		char clientSteamV2ID[32]; GetClientAuthId(client, AuthId_Steam2, clientSteamV2ID, sizeof(clientSteamV2ID));
		char clientIP[46]; GetClientIP(client, clientIP, sizeof(clientIP), true);

		char targetSteamID[32]; GetClientAuthId(target, AuthId_SteamID64, targetSteamID, sizeof(targetSteamID));
		char targetSteamV2ID[32]; GetClientAuthId(target, AuthId_Steam2, targetSteamV2ID, sizeof(targetSteamV2ID));
		char targetIP[46]; GetClientIP(target, targetIP, sizeof(targetIP), true);

		char config[256]; GetConVarString(cvarConfig, config, sizeof(config));

		DataPack dataPack = new DataPack();
		dataPack.WriteCell(client);
		dataPack.WriteString(clientSteamV2ID);
		dataPack.WriteCell(target);
		dataPack.WriteString(targetSteamV2ID);
		dataPack.WriteString(reason);
		dataPack.Reset();

		int escapedConfigLength = strlen(config) * 2 + 1;
		char[] escapedConfig    = new char[escapedConfigLength];
		SQL_EscapeString(hDatabase, config, escapedConfig, escapedConfigLength);

		int escapedClientSteamIDLength = strlen(clientSteamID) * 2 + 1;
		char[] escapedClientSteamID    = new char[escapedClientSteamIDLength];
		SQL_EscapeString(hDatabase, clientSteamID, escapedClientSteamID, escapedClientSteamIDLength);

		int escapedClientIPLength = strlen(clientIP) * 2 + 1;
		char[] escapedClientIP    = new char[escapedClientIPLength];
		SQL_EscapeString(hDatabase, clientIP, escapedClientIP, escapedClientIPLength);

		int escapedTargetIPLength = strlen(targetIP) * 2 + 1;
		char[] escapedTargetIP    = new char[escapedTargetIPLength];
		SQL_EscapeString(hDatabase, targetIP, escapedTargetIP, escapedTargetIPLength);

		int escapedTargetSteamIDLength = strlen(targetSteamID) * 2 + 1;
		char[] escapedTargetSteamID    = new char[escapedTargetSteamIDLength];
		SQL_EscapeString(hDatabase, targetSteamID, escapedTargetSteamID, escapedTargetSteamIDLength);

		int escapedServerIPLength = strlen(serverIP) * 2 + 1;
		char[] escapedServerIP    = new char[escapedServerIPLength];
		SQL_EscapeString(hDatabase, serverIP, escapedServerIP, escapedServerIPLength);

		int escapedReasonLength = strlen(reason) * 2 + 1;
		char[] escapedReason    = new char[escapedReasonLength];
		SQL_EscapeString(hDatabase, reason, escapedReason, escapedReasonLength);

		char query[512]; Format(query, sizeof(query), "INSERT INTO reports(config, client_steam_id, client_ip, target_steam_id, target_ip, server_ip, reason) VALUES ('%s', '%s', '%s', '%s', '%s', '%s', '%s')", escapedConfig, escapedClientSteamID, escapedClientIP, escapedTargetSteamID, escapedTargetIP, escapedServerIP, escapedReason);
		hDatabase.Query(PostReportPlayerQuery, query, dataPack);

		lastUsed[client]     = GetGameTime();
		lastReported[target] = GetGameTime();
	}
	else
	{
		MC_PrintToChat(client, "{#a020f0}[{#00ffff}Report{#a020f0}] {#ff5555}Failed to submit report, please try again.");

		// Database might have been down at startup so we try to get a new connection again
		Database.Connect(SetDatabase, SQL_CheckConfig("reports") ? "reports" : "default");
	}
}

public void PostReportPlayerQuery(Database db, DBResultSet results, const char[] error, any data)
{
	DataPack dataPack = view_as<DataPack>(data);

	int  client = dataPack.ReadCell();
	char clientSteamV2ID[32]; dataPack.ReadString(clientSteamV2ID, sizeof(clientSteamV2ID));
	int  target = dataPack.ReadCell();
	char targetSteamV2ID[32]; dataPack.ReadString(targetSteamV2ID, sizeof(targetSteamV2ID));
	char reason[256]; dataPack.ReadString(reason, sizeof(reason));

	CloseHandle(dataPack);

	if (db == null || results == null || error[0] != '\0')
	{
		LogError("Query failed: %s", error);
		MC_PrintToChat(client, "{#a020f0}[{#00ffff}Report{#a020f0}] {#ff5555}Failed to submit report, please try again.");
		return;
	}

	MC_PrintToChat(client, "{#a020f0}[{#00ffff}Report{#a020f0}] {#aaaaaa}Reported player {#5555ff}%N {#aaaaaa}for {#555555}%s{#aaaaaa}.", target, reason);
	LogMessage("%N[%s] reported player %N[%s] for %s.", client, clientSteamV2ID, target, targetSteamV2ID, reason);

	for (int i = 1; i <= MaxClients; i++)
	{
		if (IsClientConnected(i) && client != i && CheckCommandAccess(i, "sm_admin", ADMFLAG_GENERIC))
		{
			MC_PrintToChat(i, "{#a020f0}[{#00ffff}Report{#a020f0}] {#5555ff}%N{#555555}[{#aaaaaa}%s{#555555}] {#aaaaaa}reported player {#5555ff}%N{#555555}[{#aaaaaa}%s{#555555}] {#aaaaaa}for {#555555}%s{#aaaaaa}.", client, clientSteamV2ID, target, targetSteamV2ID, reason)
		}
	}
}

public void ReportServer(int client, const char[] reason)
{
	if (lastServerReport != 0.0 && lastServerReport + GetConVarFloat(cvarReportedDelay) > GetGameTime())
	{
		MC_PrintToChat(client, "{#a020f0}[{#00ffff}Report{#a020f0}] {#aaaaaa}A report for the server has already been issued.");
		return;
	}

	if (hDatabase != null)
	{
		char clientSteamID[32]; GetClientAuthId(client, AuthId_SteamID64, clientSteamID, sizeof(clientSteamID));
		char clientSteamV2ID[32]; GetClientAuthId(client, AuthId_Steam2, clientSteamV2ID, sizeof(clientSteamV2ID));
		char clientIP[46]; GetClientIP(client, clientIP, sizeof(clientIP), true);

		char config[256]; GetConVarString(cvarConfig, config, sizeof(config));

		DataPack dataPack = new DataPack();
		dataPack.WriteCell(client);
		dataPack.WriteString(clientSteamV2ID);
		dataPack.WriteString(reason);
		dataPack.Reset();

		int escapedConfigLength = strlen(config) * 2 + 1;
		char[] escapedConfig    = new char[escapedConfigLength];
		SQL_EscapeString(hDatabase, config, escapedConfig, escapedConfigLength);

		int escapedClientSteamIDLength = strlen(clientSteamID) * 2 + 1;
		char[] escapedClientSteamID    = new char[escapedClientSteamIDLength];
		SQL_EscapeString(hDatabase, clientSteamID, escapedClientSteamID, escapedClientSteamIDLength);

		int escapedClientIPLength = strlen(clientIP) * 2 + 1;
		char[] escapedClientIP    = new char[escapedClientIPLength];
		SQL_EscapeString(hDatabase, clientIP, escapedClientIP, escapedClientIPLength);

		int escapedServerIPLength = strlen(serverIP) * 2 + 1;
		char[] escapedServerIP    = new char[escapedServerIPLength];
		SQL_EscapeString(hDatabase, serverIP, escapedServerIP, escapedServerIPLength);

		int escapedReasonLength = strlen(reason) * 2 + 1;
		char[] escapedReason    = new char[escapedReasonLength];
		SQL_EscapeString(hDatabase, reason, escapedReason, escapedReasonLength);

		char query[512]; Format(query, sizeof(query), "INSERT INTO reports(config, client_steam_id, client_ip, server_ip, reason) VALUES ('%s', '%s', '%s', '%s', '%s')", escapedConfig, escapedClientSteamID, escapedClientIP, escapedServerIP, escapedReason);
		hDatabase.Query(PostReportServerQuery, query, dataPack);

		lastUsed[client] = GetGameTime();
		lastServerReport = GetGameTime();
	}
	else
	{
		MC_PrintToChat(client, "{#a020f0}[{#00ffff}Report{#a020f0}] {#ff5555}Failed to submit report, please try again.");

		// Database might have been down at startup so we try to get a new connection again
		Database.Connect(SetDatabase, SQL_CheckConfig("reports") ? "reports" : "default");
	}
}

public void PostReportServerQuery(Database db, DBResultSet results, const char[] error, any data)
{
	DataPack dataPack = view_as<DataPack>(data);

	int  client = dataPack.ReadCell();
	char clientSteamV2ID[32]; dataPack.ReadString(clientSteamV2ID, sizeof(clientSteamV2ID));
	char reason[256]; dataPack.ReadString(reason, sizeof(reason));

	CloseHandle(dataPack);

	if (db == null || results == null || error[0] != '\0')
	{
		LogError("Query failed: %s", error);
		MC_PrintToChat(client, "{#a020f0}[{#00ffff}Report{#a020f0}] {#ff5555}Failed to submit report, please try again.");
		return;
	}

	MC_PrintToChat(client, "{#a020f0}[{#00ffff}Report{#a020f0}] {#aaaaaa}Reported a {#5555ff}Server Issue {#aaaaaa}: {#555555}%s{#aaaaaa}.", reason);
	LogMessage("%N[%s] reported a server issue: %s.", client, clientSteamV2ID, reason);

	for (int i = 1; i <= MaxClients; i++)
	{
		if (IsClientConnected(i) && client != i && CheckCommandAccess(i, "sm_admin", ADMFLAG_GENERIC))
		{
			MC_PrintToChat(i, "{#a020f0}[{#00ffff}Report{#a020f0}] {#5555ff}%N{#555555}[{#aaaaaa}%s{#555555}] {#aaaaaa}reported a {#5555ff}Server Issue {#aaaaaa}: {#555555}%s{#aaaaaa}.", client, clientSteamV2ID, reason)
		}
	}
}

public int AdminOnlineHandler(Menu menu, MenuAction action, int client, int item)
{
	if (action == MenuAction_Select)
	{
		if (item == 0) OpenTargetsMenu(client);
	}
	else if (action == MenuAction_End)
	{
		delete menu;
	}
}

public void OpenAdminOnlineMenu(int client)
{
	Menu menu = new Menu(AdminOnlineHandler, MENU_ACTIONS_ALL);
	menu.SetTitle("An admin is online. Continue?");
	menu.ExitBackButton = false;
	menu.AddItem("Continue", "Continue");
	menu.AddItem("Cancel", "Cancel");
	menu.Display(client, MENU_TIME_FOREVER);
}

public int ChooseTargetHandler(Menu menu, MenuAction action, int client, int item)
{
	if (action == MenuAction_Select)
	{
		char target[12]; menu.GetItem(item, target, sizeof(target));
		targets[client] = StringToInt(target);
		OpenReasonsMenu(client, targets[client] == -1);
	}
	else if (action == MenuAction_End)
	{
		delete menu;
	}
}

public void OpenTargetsMenu(int client)
{
	Menu menu = new Menu(ChooseTargetHandler, MENU_ACTIONS_ALL);
	menu.SetTitle("Report Player");
	menu.ExitBackButton = false;
	AddTargetsToMenu(menu);
	menu.Display(client, MENU_TIME_FOREVER);
}

public int ChooseReasonHandler(Menu menu, MenuAction action, int client, int item)
{
	if (action == MenuAction_Select)
	{
		char reason[32]; menu.GetItem(item, reason, sizeof(reason));
		reasons[client] = reason;
		OpenConfirmationMenu(client);
	}
	else if (action == MenuAction_Cancel && item == MenuCancel_ExitBack)
	{
		OpenTargetsMenu(client);
	}
	else if (action == MenuAction_End)
	{
		delete menu;
	}
}

public void OpenReasonsMenu(int client, bool server)
{
	Menu menu = new Menu(ChooseReasonHandler, MENU_ACTIONS_ALL);
	menu.SetTitle("Choose Reason");
	menu.ExitBackButton = true;
	if (server) AddServerReasonsToMenu(menu);
	else AddPlayerReasonsToMenu(menu);
	menu.Display(client, MENU_TIME_FOREVER);
}

public int ConfirmationHandler(Menu menu, MenuAction action, int client, int item)
{
	if (action == MenuAction_Select)
	{
		if (item == 0)
		{
			int target = targets[client];
			if (target == -1) ReportServer(client, reasons[client]);
			else ReportPlayer(client, target, reasons[client]);
		}
	}
	else if (action == MenuAction_End)
	{
		delete menu;
	}
}

public void OpenConfirmationMenu(int client)
{
	Menu menu = new Menu(ConfirmationHandler, MENU_ACTIONS_ALL);
	menu.SetTitle("Warning: Abuse of the report system will lead to punishment and\n the possible banning of your account for minimum one week. Continue?");
	menu.ExitBackButton = false;
	menu.AddItem("Continue", "Continue");
	menu.AddItem("Cancel", "Cancel");
	menu.Display(client, MENU_TIME_FOREVER);
}

public void AddTargetsToMenu(Menu menu)
{
	char target[12];
	char name[MAX_NAME_LENGTH];

	menu.AddItem("-1", "Server");
	for (int i = 1; i <= MaxClients; i++)
	{
		if (!IsClientConnected(i) || IsClientInKickQueue(i) || IsFakeClient(i) || IsAdmin(i))
		{
			continue;
		}

		IntToString(i, target, sizeof(target));
		GetClientName(i, name, sizeof(name));

		char targetSteamV2ID[32];
		if (!GetClientAuthId(i, AuthId_Steam2, targetSteamV2ID, sizeof(targetSteamV2ID)))
		{
			continue;
		}

		char display[MAX_NAME_LENGTH + sizeof(targetSteamV2ID) + 3];
		StrCat(display, sizeof(display), name);
		StrCat(display, sizeof(display), " [");
		StrCat(display, sizeof(display), targetSteamV2ID);
		StrCat(display, sizeof(display), "]");

		menu.AddItem(target, display);
	}
}

public void AddPlayerReasonsToMenu(Menu menu)
{
	menu.AddItem("Hacking", "Hacking");
	menu.AddItem("Chat/Mic Spam", "Chat/Mic Spam");
	menu.AddItem("Advertising Links", "Advertising Links");
	menu.AddItem("Admin Impersonation", "Admin Impersonation");
	menu.AddItem("Porn/Gore Sprays", "Porn/Gore Sprays");
	menu.AddItem("Exploiting", "Exploiting");
	menu.AddItem("Abusing VIP", "Abusing VIP");
}

public void AddServerReasonsToMenu(Menu menu)
{
	menu.AddItem("Severe Lag", "Severe Lag");
	menu.AddItem("Server Mod Broken/Glitched", "Server Mod Broken/Glitched");
}

public bool IsAdminOnline()
{
	for (int i = 1; i <= MaxClients; i++)
	{
		if (IsClientConnected(i) && IsAdmin(i))
		{
			return true;
		}
	}
	return false;
}

public bool IsAdmin(int client)
{
	return CheckCommandAccess(client, "sm_admin", ADMFLAG_GENERIC);
}
