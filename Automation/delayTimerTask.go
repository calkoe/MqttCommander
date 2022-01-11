package Automation

func delayTimerTask(AutomationId uint) {
	automation, ok := Get(AutomationId)
	for ok {
		<-automation.Delay_Timer.C
		automation, ok := Get(AutomationId)
		if ok {
			StartTriggerFunc(automation, true)
			SetDelayActive(AutomationId, false)
		} else {
			break
		}
	}
}
