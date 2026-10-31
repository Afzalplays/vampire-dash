import { prisma } from "../lib/prisma.js";
import { sendMail } from "../lib/mailer.js";

export type NotificationEvent =
  | "SERVER_CREATED"
  | "SERVER_EXPIRING_7_DAYS"
  | "SERVER_EXPIRING_1_DAY"
  | "SERVER_SUSPENDED"
  | "SERVER_RESTORED"
  | "PAYMENT_RECEIVED"
  | "PAYMENT_FAILED"
  | "PASSWORD_CHANGED"
  | "SECURITY_ALERT";

const templates: Record<NotificationEvent, (data: Record<string, string>) => { subject: string; text: string }> = {
  SERVER_CREATED: d => ({ subject: "Your Minecraft server is ready", text: `Your server "${d.serverName ?? "Minecraft server"}" has been created. You can now manage it from Vampire Dash.` }),
  SERVER_EXPIRING_7_DAYS: d => ({ subject: "Your server expires in 7 days", text: `Your server "${d.serverName ?? "Minecraft server"}" expires in 7 days. Renew it to avoid interruption.` }),
  SERVER_EXPIRING_1_DAY: d => ({ subject: "Action required: server expires tomorrow", text: `Your server "${d.serverName ?? "Minecraft server"}" expires in 1 day. Renew now to avoid suspension.` }),
  SERVER_SUSPENDED: d => ({ subject: "Your Minecraft server has been suspended", text: `Your server "${d.serverName ?? "Minecraft server"}" has been suspended. Sign in to review the reason and next steps.` }),
  SERVER_RESTORED: d => ({ subject: "Your Minecraft server is active again", text: `Your server "${d.serverName ?? "Minecraft server"}" has been restored.` }),
  PAYMENT_RECEIVED: d => ({ subject: "Payment received", text: `We received your payment of ${d.amount ?? "the stated amount"} for order ${d.orderId ?? "N/A"}.` }),
  PAYMENT_FAILED: d => ({ subject: "Payment could not be confirmed", text: `We could not confirm payment for order ${d.orderId ?? "N/A"}. Please review your billing page.` }),
  PASSWORD_CHANGED: () => ({ subject: "Your password was changed", text: "Your Vampire Dash password was changed. If this was not you, contact support immediately." }),
  SECURITY_ALERT: d => ({ subject: "Vampire Dash security alert", text: d.message ?? "A security-related event occurred on your account." }),
};

export async function queueEmailNotification(userId: string, event: NotificationEvent, data: Record<string, string> = {}, dedupeKey?: string) {
  const user = await prisma.user.findUnique({ where: { id: userId }, select: { email: true } });
  if (!user) throw new Error("Notification recipient not found");
  const template = templates[event](data);
  const row = await prisma.emailNotification.create({
    data: { userId, recipient: user.email, event, subject: template.subject, bodyText: template.text, dedupeKey },
  });
  try {
    await sendMail({ to: row.recipient, subject: row.subject, text: row.bodyText });
    await prisma.emailNotification.update({ where: { id: row.id }, data: { status: "SENT", sentAt: new Date() } });
  } catch (error) {
    await prisma.emailNotification.update({ where: { id: row.id }, data: { status: "FAILED", lastError: error instanceof Error ? error.message.slice(0, 500) : "Delivery failed", attempts: { increment: 1 } } });
    throw error;
  }
}
