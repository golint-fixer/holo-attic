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

	"./holo"
)

//Note: This line is parsed by the Makefile to get the version string. If you
//change the format, adjust the Makefile too.
var version string = "v0.3-pre"

func main() {
	//a command word must be given as first argument
	if len(os.Args) < 2 {
		commandHelp()
		return
	}

	//check that it is a known command word
	var command func(holo.ConfigFiles)
	switch os.Args[1] {
	case "apply":
		command = commandApply
	case "scan":
		command = commandScan
	case "version", "--version":
		fmt.Println(version)
		return
	default:
		commandHelp()
		return
	}

	//scan the repo
	files := holo.ScanRepo()
	if files == nil {
		//some fatal error occurred while scanning the repo - it was already
		//reported, so just exit
		return
	}

	//execute command
	command(files)
}

func commandHelp() {
	program := os.Args[0]
	fmt.Printf("Usage: %s <operation> [...]\nOperations:\n", program)
	fmt.Printf("    %s apply [--force] [file(s)]\n", program)
	fmt.Printf("    %s scan\n", program)
	fmt.Printf("\nSee `man 8 holo` for details.\n")
}

func commandApply(files holo.ConfigFiles) {
	//parse arguments after "holo apply" (either files or "--force")
	withForce := false
	withFiles := false
	targetFiles := make(map[string]bool)

	args := os.Args[2:]
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			if arg == "--force" {
				withForce = true
			} else {
				fmt.Println("Unrecognized option: " + arg)
				return
			}
		} else {
			targetFiles[arg] = true
			withFiles = true
		}
	}

	//apply all files found in the repo (or only some if the args contain a limited subset)
	for _, file := range files {
		if !withFiles || targetFiles[file.TargetPath()] {
			holo.Apply(file, withForce)
		}
	}
}

func commandScan(files holo.ConfigFiles) {
	//report scan results
	fmt.Println()

	for _, file := range files {
		fmt.Printf("\x1b[1m%s\x1b[0m\n", file.TargetPath())
		fmt.Printf("    store at %s\n", file.BackupPath())
		repoFiles := file.RepoFiles()
		for _, repoFile := range repoFiles {
			fmt.Printf("    %8s %s\n", repoFile.ApplicationStrategy(), repoFile.Path())
		}
		fmt.Println("")
	}
}
