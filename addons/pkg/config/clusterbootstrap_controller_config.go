// Copyright 2022 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package config

// Config contains ClusterBootstrap controller configuration information.
type ClusterBootstrapControllerConfig struct {
	HTTPProxyClusterClassVarName   string
	HTTPSProxyClusterClassVarName  string
	NoProxyClusterClassVarName     string
	ProxyCACertClusterClassVarName string
}
