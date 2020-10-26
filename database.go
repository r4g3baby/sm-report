package main

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"github.com/rs/zerolog/log"
	"time"
)

var (
	database *sql.DB

	fetchAllReports  *sql.Stmt
	fetchUserReports *sql.Stmt
	updateReport     *sql.Stmt
)

type Report struct {
	ID              int
	Config          string
	ClientSteamID   uint64
	TargetSteamID   uint64
	Reason          string
	Created         time.Time
	PreviousReports []Report
}

func setUpDatabase() {
	db, err := sql.Open("mysql", config.DSN)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to open database")
	}

	db.SetConnMaxLifetime(time.Minute * 3)
	db.SetMaxOpenConns(3)
	db.SetMaxIdleConns(5)

	if err := db.Ping(); err != nil {
		log.Fatal().Err(err).Msg("could not establish a database connection")
	}

	if _, err = db.Exec(`
create table if not exists reports
(
    id            int auto_increment
        primary key,
    config        varchar(32)                            not null,
    clientSteamID bigint                                 not null,
    targetSteamID bigint                                 not null,
    reason        varchar(255)                           not null,
    handled       tinyint(1) default 0                   not null,
    created       timestamp  default current_timestamp() not null
);
	`); err != nil {
		log.Fatal().Err(err).Msg("failed to create database reports table")
	}

	stmt, err := db.Prepare("SELECT id, config, clientSteamID, targetSteamID, reason, created FROM reports WHERE handled=false")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create fetchAllReports prepared statement")
	}
	fetchAllReports = stmt

	stmt, err = db.Prepare("SELECT id, config, clientSteamID, targetSteamID, reason, created FROM reports WHERE targetSteamID=?")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create fetchUserReports prepared statement")
	}
	fetchUserReports = stmt

	stmt, err = db.Prepare("UPDATE reports SET handled=true WHERE id=?")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create updateReport prepared statement")
	}
	updateReport = stmt

	database = db
}

func getPendingReports() ([]Report, error) {
	var reports []Report

	rows, err := fetchAllReports.Query()
	if err != nil {
		return reports, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var report Report
		if err := rows.Scan(
			&report.ID,
			&report.Config,
			&report.ClientSteamID,
			&report.TargetSteamID,
			&report.Reason,
			&report.Created,
		); err != nil {
			return reports, err
		}

		uRows, err := fetchUserReports.Query(report.TargetSteamID)
		if err != nil {
			return reports, err
		}

		for uRows.Next() {
			var uReport Report
			if err := uRows.Scan(
				&uReport.ID,
				&uReport.Config,
				&uReport.ClientSteamID,
				&uReport.TargetSteamID,
				&uReport.Reason,
				&uReport.Created,
			); err != nil {
				return reports, err
			}

			if uReport.ID != report.ID {
				report.PreviousReports = append(report.PreviousReports, uReport)
			}
		}

		_ = uRows.Close()

		if _, err := updateReport.Exec(report.ID); err == nil {
			reports = append(reports, report)
		}
	}

	if err = rows.Err(); err != nil {
		return reports, err
	}

	return reports, nil
}

func closeDatabase() {
	_ = updateReport.Close()
	_ = fetchUserReports.Close()
	_ = fetchAllReports.Close()

	if err := database.Close(); err != nil {
		log.Error().Err(err).Msg("failed to safely close database connection")
	}
}
