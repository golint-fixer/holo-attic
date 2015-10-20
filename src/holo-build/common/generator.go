/*******************************************************************************
*
* Copyright 2015 Stefan Majewsky <majewsky@gmx.net>
*
* This file is part of Holo.
*
* Holo is free software: you can redistribute it and/or modify it under the
* terms of the GNU General Public License as published by the Free Software
* Foundation, either version 3 of the License, or (at your option) any later
* version.
*
* Holo is distributed in the hope that it will be useful, but WITHOUT ANY
* WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR
* A PARTICULAR PURPOSE. See the GNU General Public License for more details.
*
* You should have received a copy of the GNU General Public License along with
* Holo. If not, see <http://www.gnu.org/licenses/>.
*
*******************************************************************************/

package common

//Generator is a generic interface for the package generator implementations.
//One Generator exists for every target package format (e.g. pacman, dpkg, RPM)
//supported by holo-build.
type Generator interface {
	//Build produces the final package (usually a compressed tar file) in the
	//return argument. When it is called, all files and directories contained
	//in the package definition have already been materialized in the temporary
	//directory specified in the second argument.
	//
	//For example, if pkg contains the file
	//
	//    [[file]]
	//    name = "/etc/foo.conf"
	//    content = "xxx"
	//    mode = "0400"
	//    owner = "root"
	//    group = "root"
	//
	//Then this file has already been placed at `rootPath+"/etc/foo.conf"` with
	//the right content, ownership, and permissions. The generator usually just
	//has to write the package metadata into the temporary directory, tar the
	//directory and compress it.
	Build(pkg *Package, rootPath string) ([]byte, error)
}
