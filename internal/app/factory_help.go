package app

import "fmt"

const (
	factoryCommandCardSubmit     = "card submit {card-path}"
	factoryCommandCardReview     = "card review {card-id}"
	factoryCommandCardApprove    = "card review {card-id} --approve --confidence observed --rationale \"{manual rationale}\""
	factoryCommandPackBuild      = "pack build {cards-dir} {output-pack}"
	factoryCommandPackPublish    = "pack publish {pack-path}"
	factoryCommandPackSync       = "pack sync [{source-path}]"
	factoryCommandPackMatchDebug = "pack match-debug \"{diagnose text}\""
	factoryCommandStatus         = "status"
)

func factoryCommandPrefix(surface factorySurface) string {
	if surface == factorySurfaceCLI {
		return "mscli factory"
	}
	return "/factory"
}

func factoryCommand(surface factorySurface, suffix string) string {
	return factoryCommandPrefix(surface) + " " + suffix
}

func renderFactoryHelpForSurface(surface factorySurface) string {
	if surface == factorySurfaceCLI {
		return fmt.Sprintf("Factory commands:\n\nCard workflow:\n  %s\n  %s\n  %s\n\nPack workflow:\n  %s\n  %s\n  %s\n  %s\n\nStatus:\n  %s",
			factoryCommand(surface, factoryCommandCardSubmit),
			factoryCommand(surface, factoryCommandCardReview),
			factoryCommand(surface, factoryCommandCardApprove),
			factoryCommand(surface, factoryCommandPackBuild),
			factoryCommand(surface, factoryCommandPackPublish),
			factoryCommand(surface, factoryCommandPackSync),
			factoryCommand(surface, factoryCommandPackMatchDebug),
			factoryCommand(surface, factoryCommandStatus),
		)
	}
	return renderFactoryHelp()
}

func renderFactoryCardHelpForSurface(surface factorySurface) string {
	if surface == factorySurfaceCLI {
		return fmt.Sprintf("Factory card commands:\n  %s\n  %s\n  %s",
			factoryCommand(surface, factoryCommandCardSubmit),
			factoryCommand(surface, factoryCommandCardReview),
			factoryCommand(surface, factoryCommandCardApprove),
		)
	}
	return renderFactoryCardHelp()
}

func renderFactoryPackHelpForSurface(surface factorySurface) string {
	if surface == factorySurfaceCLI {
		return fmt.Sprintf("Factory pack commands:\n  %s\n  %s\n  %s\n  %s",
			factoryCommand(surface, factoryCommandPackBuild),
			factoryCommand(surface, factoryCommandPackPublish),
			factoryCommand(surface, factoryCommandPackSync),
			factoryCommand(surface, factoryCommandPackMatchDebug),
		)
	}
	return renderFactoryPackHelp()
}

func factoryUsageError(surface factorySurface, command string) string {
	switch command {
	case "status":
		return "Usage: " + factoryCommand(surface, factoryCommandStatus)
	case "card submit":
		return "Usage: " + factoryCommand(surface, factoryCommandCardSubmit)
	case "card review":
		return "Usage: " + factoryCommandPrefix(surface) + " card review {card-id} [--approve --confidence observed --rationale \"{manual rationale}\"]"
	case "pack build":
		return "Usage: " + factoryCommand(surface, factoryCommandPackBuild)
	case "pack publish":
		return "Usage: " + factoryCommand(surface, factoryCommandPackPublish)
	case "pack sync":
		return "Usage: " + factoryCommand(surface, factoryCommandPackSync)
	case "pack match-debug":
		return "Usage: " + factoryCommand(surface, factoryCommandPackMatchDebug)
	default:
		return renderFactoryHelpForSurface(surface)
	}
}

func renderFactoryHelp() string {
	return fmt.Sprintf("Factory commands:\n\nCard workflow:\n  %s\n  %s\n  %s\n  %s\n\nPack workflow:\n  %s\n  %s\n  %s\n  %s\n\nStatus:\n  %s",
		factoryCommand(factorySurfaceTUI, "card create"),
		factoryCommand(factorySurfaceTUI, factoryCommandCardSubmit),
		factoryCommand(factorySurfaceTUI, factoryCommandCardReview),
		factoryCommand(factorySurfaceTUI, factoryCommandCardApprove),
		factoryCommand(factorySurfaceTUI, factoryCommandPackBuild),
		factoryCommand(factorySurfaceTUI, factoryCommandPackPublish),
		factoryCommand(factorySurfaceTUI, factoryCommandPackSync),
		factoryCommand(factorySurfaceTUI, factoryCommandPackMatchDebug),
		factoryCommand(factorySurfaceTUI, factoryCommandStatus),
	)
}

func renderFactoryCardHelp() string {
	return fmt.Sprintf("Factory card commands:\n  %s\n  %s\n  %s\n  %s",
		factoryCommand(factorySurfaceTUI, "card create"),
		factoryCommand(factorySurfaceTUI, factoryCommandCardSubmit),
		factoryCommand(factorySurfaceTUI, factoryCommandCardReview),
		factoryCommand(factorySurfaceTUI, factoryCommandCardApprove),
	)
}

func renderFactoryPackHelp() string {
	return fmt.Sprintf("Factory pack commands:\n  %s\n  %s\n  %s\n  %s",
		factoryCommand(factorySurfaceTUI, factoryCommandPackBuild),
		factoryCommand(factorySurfaceTUI, factoryCommandPackPublish),
		factoryCommand(factorySurfaceTUI, factoryCommandPackSync),
		factoryCommand(factorySurfaceTUI, factoryCommandPackMatchDebug),
	)
}
