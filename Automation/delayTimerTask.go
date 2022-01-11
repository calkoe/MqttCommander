package Automation

func delayTimerTask(AutomationId uint) {
	automation, ok := Get(AutomationId)
	for ok {
		<-automation.Delay_Timer.C
		_, ok := Get(AutomationId)
		if ok {
			StartActions(AutomationId, true)
			SetDelayActive(AutomationId, false)
		} else {
			break
		}
	}
}
