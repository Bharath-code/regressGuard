package failures

type Actionable struct {
	Title       string `json:"title"`
	Cause       string `json:"cause"`
	Next        string `json:"next"`
	MoreContext string `json:"moreContext"`
}

func (e Actionable) Error() string {
	return e.Title + "\n\nLikely cause:\n  " + e.Cause +
		"\n\nRun:\n  " + e.Next +
		"\n\nNeed more context:\n  " + e.MoreContext
}

func MissingConfig() Actionable {
	return Actionable{
		Title:       "regressguard snapshot failed: no config found.",
		Cause:       "RegressGuard has not been initialized for this project.",
		Next:        "regressguard init",
		MoreContext: "regressguard snapshot --help",
	}
}

func MissingSnapshot() Actionable {
	return Actionable{
		Title:       "regressguard check failed: no snapshot found.",
		Cause:       "No known-good baseline has been recorded yet.",
		Next:        "regressguard snapshot",
		MoreContext: "regressguard check --help",
	}
}

func ServerUnavailable(url string) Actionable {
	return Actionable{
		Title:       "regressguard route check failed: dev server is unreachable.",
		Cause:       "The configured server is not running at " + url + ".",
		Next:        "npm run dev",
		MoreContext: "regressguard doctor",
	}
}

func MissingTestCommand() Actionable {
	return Actionable{
		Title:       "regressguard test run failed: no test command configured.",
		Cause:       "RegressGuard could not infer how to run this project's tests.",
		Next:        "regressguard config set testCommand \"npm test\"",
		MoreContext: "regressguard config --help",
	}
}

func ProjectRootMissing() Actionable {
	return Actionable{
		Title:       "regressguard init failed: no project root found.",
		Cause:       "This directory is not inside a package.json or git repository.",
		Next:        "cd <your-project> && regressguard init",
		MoreContext: "regressguard init --help",
	}
}

func DevServerURLRequired() Actionable {
	return Actionable{
		Title:       "regressguard init failed: dev server URL is required in non-interactive mode.",
		Cause:       "The default http://localhost:3000 server was not reachable, and scripts cannot be prompted.",
		Next:        "regressguard init --server-url http://localhost:3000",
		MoreContext: "regressguard init --help",
	}
}

func ConfigExists(path string) Actionable {
	return Actionable{
		Title:       "regressguard init failed: config already exists.",
		Cause:       "RegressGuard will not overwrite " + path + " without confirmation.",
		Next:        "regressguard init --yes",
		MoreContext: "regressguard init --help",
	}
}
