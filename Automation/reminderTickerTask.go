package Automation

func reminderTickerTask(AutomationId uint) {
	automation, ok := Get(AutomationId)
	for ok {
		<-automation.Reminder_Ticker.C
		_, ok := Get(AutomationId)
		if ok {
			StartTriggerFunc(AutomationId, true)
		} else {
			break
		}
	}
}
