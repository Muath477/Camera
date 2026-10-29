"use client";

import { motion } from "motion/react";

// Each route change fades and lifts the page in. Kept short so navigation
// feels immediate rather than animated.
export default function Template({ children }: { children: React.ReactNode }) {
  return (
    <motion.div initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.28, ease: [0.16, 1, 0.3, 1] }}>
      {children}
    </motion.div>
  );
}
