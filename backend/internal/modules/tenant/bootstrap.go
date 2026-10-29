package tenant

import (
	"context"

	"github.com/google/uuid"

	"provider_saas/internal/platform/db"
)

// Default onboarding data inserted when a tenant registers. These rows belong to the
// storefront / inbox / reminder modules' tables; tenant bootstrap is the one place that
// seeds them so a new provider is useful in the first minute (goal.md success metric).

type defaultTemplate struct{ key, name, category, body string }

var defaultTemplates = []defaultTemplate{
	{"reminder.before_3d", "মেয়াদ শেষের ৩ দিন আগে", "reminder",
		"প্রিয় {{customer}}, আপনার {{product}} সাবস্ক্রিপশনের মেয়াদ {{expiry}} তারিখে শেষ হবে। রিনিউ করতে {{amount}} পাঠিয়ে TrxID দিন। ধন্যবাদ!"},
	{"reminder.on_expiry", "মেয়াদ শেষের দিন", "reminder",
		"প্রিয় {{customer}}, আজ আপনার {{product}} এর মেয়াদ শেষ হচ্ছে। সার্ভিস চালু রাখতে আজই রিনিউ করুন: {{link}}"},
	{"reminder.after_2d", "মেয়াদ শেষের ২ দিন পর", "winback",
		"প্রিয় {{customer}}, আপনার {{product}} এর মেয়াদ ২ দিন আগে শেষ হয়েছে। আবার চালু করতে চাইলে এই মেসেজের রিপ্লাই দিন।"},
	{"delivery.default", "ডেলিভারি মেসেজ", "delivery",
		"প্রিয় {{customer}}, আপনার {{product}} প্রস্তুত ✅ মেয়াদ: {{expiry}}\n{{credentials}}\nকোনো সমস্যা হলে জানাবেন।"},
	{"invoice.default", "ইনভয়েস", "invoice",
		"প্রিয় {{customer}}, আপনার ইনভয়েস {{invoice}} — মোট {{amount}}। বিস্তারিত: {{link}}"},
	{"quick_reply.price", "প্রাইস লিস্ট", "quick_reply",
		"আমাদের সব প্যাকেজ ও দাম দেখুন: {{link}}"},
	{"quick_reply.payment", "পেমেন্ট নির্দেশনা", "quick_reply",
		"bKash/Nagad এ Send Money করে TrxID টি এখানে পাঠান। পেমেন্ট ভেরিফাই হলেই ডেলিভারি দেওয়া হবে।"},
}

type defaultRule struct {
	name, trigger, templateKey string
	offsetDays                 int
}

var defaultRules = []defaultRule{
	{"মেয়াদ শেষের ৩ দিন আগে", "before_expiry", "reminder.before_3d", 3},
	{"মেয়াদ শেষের দিন", "on_expiry", "reminder.on_expiry", 0},
	{"মেয়াদ শেষের ২ দিন পর", "after_expiry", "reminder.after_2d", 2},
}

// bootstrap inserts the default storefront, message templates and reminder rules.
func bootstrap(ctx context.Context, q db.Querier, t Tenant) error {
	if _, err := q.Exec(ctx, `INSERT INTO storefronts (tenant_id, slug, display_name, payment_instructions)
		VALUES ($1, $2, $3, $4)`, t.ID, t.Slug, t.Name,
		"bKash/Nagad এ Send Money করে TrxID দিন। Send money via bKash/Nagad and share the TrxID."); err != nil {
		return err
	}
	tplIDs := map[string]uuid.UUID{}
	for _, tp := range defaultTemplates {
		var id uuid.UUID
		if err := q.QueryRow(ctx, `INSERT INTO message_templates (tenant_id, key, name, body, category)
			VALUES ($1,$2,$3,$4,$5) RETURNING id`, t.ID, tp.key, tp.name, tp.body, tp.category).Scan(&id); err != nil {
			return err
		}
		tplIDs[tp.key] = id
	}
	for _, r := range defaultRules {
		tid := tplIDs[r.templateKey]
		if _, err := q.Exec(ctx, `INSERT INTO reminder_rules (tenant_id, name, trigger, offset_days, template_id)
			VALUES ($1,$2,$3,$4,$5)`, t.ID, r.name, r.trigger, r.offsetDays, tid); err != nil {
			return err
		}
	}
	return nil
}
