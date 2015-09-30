/*******************************************************************************
*
*   Copyright 2015 Stefan Majewsky <majewsky@gmx.net>
*
*   This program is free software; you can redistribute it and/or modify it
*   under the terms of the GNU General Public License as published by the Free
*   Software Foundation; either version 2 of the License, or (at your option)
*   any later version.
*
*   This program is distributed in the hope that it will be useful, but WITHOUT
*   ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or
*   FITNESS FOR A PARTICULAR PURPOSE.  See the GNU General Public License for
*   more details.
*
*   You should have received a copy of the GNU General Public License along
*   with this program; if not, write to the Free Software Foundation, Inc.,
*   51 Franklin Street, Fifth Floor, Boston, MA 02110-1301, USA.
*
********************************************************************************/

package main

import (
	"fmt"
	"os"
	"strings"

	"./common"
	"./entities"
	"./files"
)

//Note: This line is parsed by the Makefile to get the version string. If you
//change the format, adjust the Makefile too.
var version = "v0.6.0"
var codename = "Providence"

func main() {
	//a command word must be given as first argument
	if len(os.Args) < 2 {
		commandHelp()
		return
	}

	//check that it is a known command word
	var command func(files.ConfigFiles, []string, entities.Entities)
	switch os.Args[1] {
	case "apply":
		command = commandApply
	case "diff":
		command = commandDiff
	case "scan":
		command = commandScan
	case "version", "--version":
		fmt.Printf("%s \"%s\"\n", version, codename)
		return
	default:
		commandHelp()
		return
	}

	//scan the repo
	configFiles, orphanedBackupFiles := files.ScanRepo()
	if configFiles == nil {
		//some fatal error occurred while scanning the repo - it was already
		//reported, so just exit
		return
	}

	//scan for entity definitions
	entities := entities.Scan()
	if entities == nil {
		//some fatal error occurred while scanning the repo - it was already
		//reported, so just exit
		return
	}

	//execute command
	command(configFiles, orphanedBackupFiles, entities)
}

func commandHelp() {
	program := os.Args[0]
	fmt.Printf("Usage: %s <operation> [...]\nOperations:\n", program)
	fmt.Printf("    %s apply [-f|--force] [target(s)]\n", program)
	fmt.Printf("    %s diff [file(s)]\n", program)
	fmt.Printf("    %s scan [-s|--short]\n", program)
	fmt.Printf("\nSee `man 8 holo` for details.\n")
}

func commandApply(configFiles files.ConfigFiles, orphanedBackupFiles []string, entities entities.Entities) {
	//parse arguments after "holo apply" (either files or "--force")
	withForce := false
	withTargets := false
	targets := make(map[string]bool)

	args := os.Args[2:]
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			switch arg {
			case "-f", "--force":
				withForce = true
			default:
				fmt.Println("Unrecognized option: " + arg)
				return
			}
		} else {
			targets[arg] = true
			withTargets = true
		}
	}

	//apply all files found in the repo (or only some if the args contain a limited subset)
	for _, file := range configFiles {
		if !withTargets || targets[file.TargetPath()] {
			files.Apply(file, withForce)
		}
	}

	//cleanup orphaned backup files
	for _, file := range orphanedBackupFiles {
		targetFile := files.NewConfigFileFromBackupPath(file).TargetPath()
		if !withTargets || targets[targetFile] {
			files.HandleOrphanedBackupFile(file)
		}
	}

	//apply all declared entities (or only some if the args contain a limites subset)
	for _, entity := range entities {
		if !withTargets || targets[entity.EntityID()] {
			entity.Apply(withForce)
		}
	}
}

func commandScan(configFiles files.ConfigFiles, orphanedBackupFiles []string, entities entities.Entities) {
	//check args
	args := os.Args[2:]
	isShort := false
	for _, arg := range args {
		//"--short" shows only the target names, not the strategy
		switch arg {
		case "-s", "--short":
			isShort = true
		default:
			fmt.Println("Unrecognized argument: " + arg)
			return
		}
	}

	//report scan results
	if !isShort {
		fmt.Println()
	}

	//report config files with repo files
	for _, file := range configFiles {
		if isShort {
			fmt.Println(file.TargetPath())
		} else {
			fmt.Printf("\x1b[1m%s\x1b[0m\n", file.TargetPath())
			fmt.Printf("    store at %s\n", file.BackupPath())
			repoFiles := file.RepoFiles()
			for _, repoFile := range repoFiles {
				fmt.Printf("    %8s %s\n", repoFile.ApplicationStrategy(), repoFile.Path())
			}
			fmt.Println()
		}
	}

	//report orphaned backup files
	if !isShort {
		for _, backupFile := range orphanedBackupFiles {
			targetFile, strategy, assessment := files.ScanOrphanedBackupFile(backupFile)
			fmt.Printf("\x1b[1m%s\x1b[0m (%s)\n", targetFile, assessment)
			fmt.Printf("    %8s %s\n", strategy, backupFile)
			fmt.Println()
		}
	}

	//report declared entities
	for _, entity := range entities {
		if isShort {
			fmt.Println(entity.EntityID())
		} else {
			fmt.Printf("\x1b[1m%s\x1b[0m\n", entity.EntityID())
			defFiles := entity.DefinitionFiles()
			for _, defFile := range defFiles {
				fmt.Printf("    found in %s\n", defFile)
			}
			if attributes := entity.Attributes(); attributes != "" {
				fmt.Printf("        with %s\n", attributes)
			}
			fmt.Println()
		}
	}
}

func commandDiff(configFiles files.ConfigFiles, orphanedBackupFiles []string, _ entities.Entities) {
	//which targets have been selected?
	if len(os.Args) == 2 {
		//no arguments given -> diff all known config files, including those
		//where repo files have been deleted
		allConfigFiles := configFiles[:]
		for _, backupFile := range orphanedBackupFiles {
			allConfigFiles = append(allConfigFiles, files.NewConfigFileFromBackupPath(backupFile))
		}
		for _, configFile := range allConfigFiles {
			output, err := configFile.RenderDiff()
			if err != nil {
				common.PrintError("Could not diff %s: %s\n", configFile.TargetPath(), err.Error())
			}
			os.Stdout.Write(output)
		}
	} else {
		args := os.Args[2:]
		for _, targetPath := range args {
			configFile := files.NewConfigFileFromTargetPath(targetPath)
			output, err := configFile.RenderDiff()
			if err != nil {
				common.PrintError("Could not diff %s: %s\n", targetPath, err.Error())
			}
			os.Stdout.Write(output)
		}
	}
}
