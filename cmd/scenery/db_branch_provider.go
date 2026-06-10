package main

import appcfg "scenery.sh/internal/app"

func dbBranchProviderForConfig(cfg appcfg.Config) dbBranchProvider {
	return postgresBranchProvider{cfg: cfg}
}
