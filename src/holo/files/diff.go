	"../../shared"
	errorReport := shared.Report{Action: "diff", Target: toPath}
	output, _ := shared.ExecProgram(&errorReport, []byte{}, "diff", "-u", fromPath, toPath)