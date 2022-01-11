package Automation

func reminderTickerTask(AutomationId uint) {
	automation, ok := Get(AutomationId)
	for ok {
		<-automation.Reminder_Ticker.C
		automation, ok := Get(AutomationId)
		if ok {
			StartTriggerFunc(automation, true)
		} else {
			break
		}
	}
}
