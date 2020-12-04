// pmm-managed
// Copyright (C) 2017 Percona LLC
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package ia

import (
	iav1beta1 "github.com/percona/pmm/api/managementpb/ia"
	"gopkg.in/reform.v1"
)

type RulesService struct {
	iav1beta1.UnimplementedRulesServer // TODO remove

	db *reform.DB
}

func NewRulesService(db *reform.DB) *RulesService {
	return &RulesService{
		db: db,
	}
}

// Check interfaces.
var (
	_ iav1beta1.RulesServer = (*RulesService)(nil)
)