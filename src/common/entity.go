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

//Entity provides a common interface for configuration entities, such as
//configuration files, user accounts and user groups.
type Entity interface {
	//EntityID returns a string that uniquely identifies the entity, usually in
	//the form "type:name". This is how the entity can be addressed as a target
	//in the argument list for "holo apply", e.g. "holo apply /etc/sudoers
	//group:foo" will apply the target file "/etc/sudoers" and the group "foo".
	//Therefore, entity IDs should not contain whitespaces or characters that
	//have a special meaning on the shell.
	EntityID() string
	//DefinitionFile returns the path to the file containing the definition of
	//this entity.
	DefinitionFiles() []string
	//Attributes returns a string describing additional attributes set for this
	//entity, alternatively an empty string.
	Attributes() string
	//Apply performs the complete application algorithm for the givne Entity.
	Apply(withForce bool)
}

//Entities holds a slice of Entity instances, and implements some methods to
//satisfy the sort.Interface interface.
type Entities []Entity

func (e Entities) Len() int           { return len(e) }
func (e Entities) Less(i, j int) bool { return e[i].EntityID() < e[j].EntityID() }
func (e Entities) Swap(i, j int)      { e[i], e[j] = e[j], e[i] }
