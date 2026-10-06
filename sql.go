package main

import (
	"database/sql"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/dronm/elkonexport/config"
	"github.com/dronm/elkonexport/logger"

	_ "github.com/denisenkom/go-mssqldb"
)

func (a *App) ConnectMSSQLWithRetry() {
	cfg := a.cfgMgr.Get()
	for {
		select {
		case <-a.ctx.Done():
			return 
		default:
		}

		if err := a.connectMSSQL(cfg.MSSQLConnStr); err != nil {
			logger.Logger.Errorf("connectMSSQL: %v; retrying...", err)
			if !waitOrCancelled(a.ctx, a.cfgMgr.ConnectRetry()) {
				return 
			}
			continue
		}
		break
	}
	logger.Logger.Info("successfully connected to MSSQL")
}

func (a *App) connectMSSQL(connStr string) error {
	db, err := sql.Open("sqlserver", connStr)
	if err != nil {
		return err
	}
	if err := db.Ping(); err != nil {
		return err
	}
	a.db = db

	return nil
}

func (a *App) productionQuery() (string, error) {
	b, err := os.ReadFile("query.sql")
	return string(b), err 
}

func (a *App) checkUnclosedProductions() error {
	var productions []int
	var prodForQuery strings.Builder
	checkProd := a.cfgMgr.CheckUnclosedProductions()

	prodCount := a.cfgMgr.QueryProductionCount()

	unclosedProductions := a.cfgMgr.Get().Production.UnclosedProductions
	for prodID, prodAdded := range unclosedProductions {
		if time.Since(prodAdded) > checkProd {
			continue // just skip
		}
		productions = append(productions, prodID)
		if prodForQuery.Len() > 0 {
			prodForQuery.WriteString(",")
		}
		prodForQuery.WriteString(fmt.Sprintf("%d", prodID))
	}

	if len(productions) == 0 {
		return nil //nothing to send
	}

	query, err := a.productionQuery()
	if err != nil  {
		return fmt.Errorf("prodForQuery(): %v", err)
	}
	query = strings.ReplaceAll(query, "{{PRODUCTION_COUNT}}", fmt.Sprintf("%d", prodCount))
	query = strings.ReplaceAll(query, "{{SIGN}}", "IN")
	query = strings.ReplaceAll(query, "{{PRODUCTION_ID}}", fmt.Sprintf("(%s) AND UretimSonuc.BitisTarihi IS NOT NULL", prodForQuery.String() ))

	logger.Logger.Debugf("retrieving unclosed productions: %v", productions)
	// logger.Logger.Debugf("q: %s", query)

	var results ElkonProductionList

	rows, err := a.db.Query(query)
	if err != nil {
		return fmt.Errorf("db.Query(): %v", err)
	}

	productions = productions[:0] //actual productions for sending, closed only
	for rows.Next() {
		var s ElkonProduction
		if err := scanToStruct(rows, &s); err != nil {
			return fmt.Errorf("scanToStruct(): %v", err)
		}
		results = append(results, s)
		productions = append(productions, s.ID)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("rows.Scan(): %v", err)
	}

	if len(results) == 0 {
		return nil
	}
	if err := a.sendClosedProductions(results, productions); err != nil {
		return err
	}

	return nil
}

func (a *App) checkNewProductions() error {
	lastProductinID := a.cfgMgr.Get().Production.LastProduction
	prodCount := a.cfgMgr.QueryProductionCount()

	query, err := a.productionQuery()
	if err != nil {
		return fmt.Errorf("checkNewProductions(): %v", err)
	}
	query = strings.ReplaceAll(query, "{{PRODUCTION_COUNT}}", fmt.Sprintf("%d", prodCount))
	query = strings.ReplaceAll(query, "{{SIGN}}", ">")
	query = strings.ReplaceAll(query, "{{PRODUCTION_ID}}", fmt.Sprintf("%d", lastProductinID))

	logger.Logger.Debugf("retrieving productions greater then %d", lastProductinID)

	var results ElkonProductionList

	rows, err := a.db.Query(query)
	if err != nil {
		return fmt.Errorf("db.Query(): %v", err)
	}

	for rows.Next() {
		var s ElkonProduction
		if err := scanToStruct(rows, &s); err != nil {
			return fmt.Errorf("scanToStruct(): %v", err)
		}
		results = append(results, s)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("rows.Scan(): %v", err)
	}

	if err := a.sendNewProductions(results, lastProductinID); err != nil {
		return err
	}

	return nil
}

func (a *App) queryLoop() {
	for {
		select {
		case <-a.ctx.Done():
			return
		default:
		}

		if err := a.checkUnclosedProductions(); err != nil {
			sendError(a, err, "")
			logger.Logger.Errorf("checkUnclosedProductions(): %v", err)
		}

		if err := a.checkNewProductions(); err != nil {
			sendError(a, err, "")
			logger.Logger.Errorf("checkNewProductions(): %v", err)
		}

		time.Sleep(a.cfgMgr.QueryInterval())
	}
}

// scanToStruct scans a single row into dest (must be a pointer to struct)
func scanToStruct(rows *sql.Rows, dest any) error {
	cols, err := rows.Columns()
	if err != nil {
		return err
	}

	destVal := reflect.ValueOf(dest).Elem()
	destType := destVal.Type()

	// map column names to struct field indexes
	fieldMap := map[string]int{}
	for i := 0; i < destType.NumField(); i++ {
		f := destType.Field(i)
		name := f.Tag.Get(elkonStructDBTag)
		if name == "" {
			name = f.Name
		}
		fieldMap[name] = i
	}

	// Prepare scan targets
	rawVals := make([]any, len(cols))
	for i := range rawVals {
		rawVals[i] = new(any)
	}

	if err := rows.Scan(rawVals...); err != nil {
		return err
	}

	for i, col := range cols {
		fieldIndex, ok := fieldMap[col]
		if !ok {
			continue // ignore unmatched columns
		}

		fv := destVal.Field(fieldIndex)
		if !fv.CanSet() {
			continue
		}

		val := reflect.ValueOf(*(rawVals[i].(*any)))
		if !val.IsValid() {
			continue
		}

		// handle pointer fields gracefully
		if fv.Kind() == reflect.Pointer {
			elemType := fv.Type().Elem()
			if val.Type().AssignableTo(elemType) {
				ptr := reflect.New(elemType)
				ptr.Elem().Set(val)
				fv.Set(ptr)
			} else if val.Type().ConvertibleTo(elemType) {
				ptr := reflect.New(elemType)
				ptr.Elem().Set(val.Convert(elemType))
				fv.Set(ptr)
			}
		} else {
			if val.Type().AssignableTo(fv.Type()) {
				fv.Set(val)
			} else if val.Type().ConvertibleTo(fv.Type()) {
				fv.Set(val.Convert(fv.Type()))
			}
		}
	}

	return nil
}

// sendClosedProductions sends newly closed productions, removes from UnclosedProductions list.
// If error occurs, server will demand resend.
func (a *App) sendClosedProductions(results ElkonProductionList, productions []int) error {
	msg := ClientMessage[ElkonProductionList]{
		Func:    responsePrefix + "Upload",
		Payload: ElkonPayload[ElkonProductionList]{ProductionSiteID: a.productionSiteID, Msg: results},
	}

	logger.Logger.Debugf("sending to server %d production(s)", len(results))

	if err := a.SendSync(msg); err != nil {
		return fmt.Errorf("SendSync(): %v", err)
	}

	//remove currently closed from server unclosed
	a.cfgMgr.RemoveFromUnclosedProductions(productions)

	return nil
}

func (a *App) sendNewProductions(results ElkonProductionList, lastProductionID int) error {
	// no result - just ping
	if len(results) == 0 {
		var n Nil = nil
		sendPayload(a, "Ping", n, "ping")
		return nil
	}

	msg := ClientMessage[ElkonProductionList]{
		Func:    responsePrefix + "Upload",
		Payload: ElkonPayload[ElkonProductionList]{ProductionSiteID: a.productionSiteID, Msg: results},
	}

	logger.Logger.Debugf("sending to server %d production(s)", len(results))

	if err := a.SendSync(msg); err != nil {
		return fmt.Errorf("SendSync(): %v", err)
	}

	// find new last productionID
	for _, prod := range results {
		if lastProductionID < prod.ID {
			lastProductionID = prod.ID
		}
		// check if unclosed
		if prod.ProductionDtEnd == nil {
			if err := a.cfgMgr.AddUnclosedProduction(prod.ID, time.Now()); err != nil {
				return fmt.Errorf("AddUnclosedProduction(): %v", err)
			}
		}
	}

	// move lastProductionID
	if err := a.cfgMgr.Update(func(c *config.Config) {
		c.Production.LastProduction = lastProductionID
	}); err != nil {
		return fmt.Errorf("a.cfgMgr.Update(): %v", err)
	}
	return nil
}
