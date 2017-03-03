// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package models

type Backup struct {
	StartTime string `json:"start_time"`
	AdminAddr string `json:"admin_addr"`

	ProtoType  string `json:"proto_type"`
	BackupAddr string `json:"backup_addr"`

	DataPath string `json:"data_path,omitempty"`

	Pid int    `json:"pid"`
	Pwd string `json:"pwd"`
	Sys string `json:"sys"`

	Hostname   string `json:"hostname"`
	DataCenter string `json:"datacenter"`
}

func (p *Backup) Encode() []byte {
	return jsonEncode(p)
}
