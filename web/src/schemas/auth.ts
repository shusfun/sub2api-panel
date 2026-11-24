import {z} from "zod";

export const sessionSchema = z.object({
  admin: z.boolean(),
  csrf: z.string(),
  step_up_enabled: z.boolean().optional().default(false),
  totp_enabled: z.boolean().optional().default(false),
});

export type SessionInfo = z.infer<typeof sessionSchema>;
