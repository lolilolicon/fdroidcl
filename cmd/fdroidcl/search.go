/* Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc> */
/* See LICENSE for licensing information */

package main

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/mvdan/fdroidcl"
	"github.com/mvdan/fdroidcl/adb"
)

var cmdSearch = &Command{
	UsageLine: "search <regexp...>",
	Short:     "Search available apps",
}

var (
	quiet     = cmdSearch.Flag.Bool("q", false, "Print package names only")
	installed = cmdSearch.Flag.Bool("i", false, "Filter installed apps")
	updates   = cmdSearch.Flag.Bool("u", false, "Filter apps with updates")
)

func init() {
	cmdSearch.Run = runSearch
}

func runSearch(args []string) {
	if *installed && *updates {
		fmt.Println("-i is redundant if -u is specified")
		cmdSearch.Flag.Usage()
	}
	var device *adb.Device
	if *installed || *updates {
		device = mustOneDevice()
	} else {
		device = maybeOneDevice()
	}
	index := mustLoadIndex()
	apps := filterAppsSearch(index.Apps, args)
	instPkgs := mustInstalled(device)
	if *installed {
		apps = filterAppsInstalled(apps, instPkgs)
	}
	if *updates {
		apps = filterAppsUpdates(apps, instPkgs)
	}
	if *quiet {
		for _, app := range apps {
			fmt.Println(app.ID)
		}
	} else {
		printApps(apps, instPkgs)
	}
}

func filterAppsSearch(apps []fdroidcl.App, terms []string) []fdroidcl.App {
	regexes := make([]*regexp.Regexp, len(terms))
	for i, term := range terms {
		regexes[i] = regexp.MustCompile(term)
	}
	var result []fdroidcl.App
	for _, app := range apps {
		fields := []string{
			strings.ToLower(app.ID),
			strings.ToLower(app.Name),
			strings.ToLower(app.Summary),
			strings.ToLower(app.Desc),
		}
		if !appMatches(fields, regexes) {
			continue
		}
		result = append(result, app)
	}
	return result
}

func appMatches(fields []string, regexes []*regexp.Regexp) bool {
fieldLoop:
	for _, field := range fields {
		for _, regex := range regexes {
			if !regex.MatchString(field) {
				continue fieldLoop
			}
		}
		return true
	}
	return false
}

func printApps(apps []fdroidcl.App, inst map[string]adb.Package) {
	maxIDLen := 0
	for _, app := range apps {
		if len(app.ID) > maxIDLen {
			maxIDLen = len(app.ID)
		}
	}
	for _, app := range apps {
		var pkg *adb.Package
		p, e := inst[app.ID]
		if e {
			pkg = &p
		}
		printApp(app, maxIDLen, pkg)
	}
}

func descVersion(app fdroidcl.App, inst *adb.Package) string {
	cur := app.CurApk
	if inst == nil {
		return fmt.Sprintf("%s (%d)", cur.VName, cur.VCode)
	}
	if inst.VCode >= cur.VCode {
		return fmt.Sprintf("%s (%d) [installed]", cur.VName, cur.VCode)
	}
	return fmt.Sprintf("%s (%d) -> %s (%d)", inst.VName, inst.VCode,
		cur.VName, cur.VCode)
}

func printApp(app fdroidcl.App, IDLen int, inst *adb.Package) {
	fmt.Printf("%s%s %s - %s\n", app.ID, strings.Repeat(" ", IDLen-len(app.ID)),
		app.Name, descVersion(app, inst))
	fmt.Printf("    %s\n", app.Summary)
}

func mustInstalled(device *adb.Device) map[string]adb.Package {
	if device == nil {
		return nil
	}
	inst, err := device.Installed()
	if err != nil {
		log.Fatalf("Could not get installed packages: %v", err)
	}
	return inst
}

func filterAppsInstalled(apps []fdroidcl.App, inst map[string]adb.Package) []fdroidcl.App {
	var result []fdroidcl.App
	for _, app := range apps {
		if _, e := inst[app.ID]; !e {
			continue
		}
		result = append(result, app)
	}
	return result
}

func filterAppsUpdates(apps []fdroidcl.App, inst map[string]adb.Package) []fdroidcl.App {
	var result []fdroidcl.App
	for _, app := range apps {
		p, e := inst[app.ID]
		if !e {
			continue
		}
		if p.VCode >= app.CurApk.VCode {
			continue
		}
		result = append(result, app)
	}
	return result
}