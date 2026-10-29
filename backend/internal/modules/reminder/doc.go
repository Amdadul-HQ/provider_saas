// Package reminder materialises reminder_rules into reminders (reminder.plan, hourly) and
// sends due ones through the inbox (reminder.dispatch, every minute). Default rules are
// seeded by tenant bootstrap on register.
package reminder
