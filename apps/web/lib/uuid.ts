function fallbackRandomBytes(size: number) {
  const bytes = new Uint8Array(size);
  for (let i = 0; i < size; i += 1) {
    bytes[i] = Math.floor(Math.random() * 256);
  }
  return bytes;
}

export function generateUUID() {
  const c = globalThis.crypto;
  if (c && typeof c.randomUUID === "function") {
    return c.randomUUID();
  }

  const bytes = c && typeof c.getRandomValues === "function" ? c.getRandomValues(new Uint8Array(16)) : fallbackRandomBytes(16);
  bytes[6] = (bytes[6] & 0x0f) | 0x40;
  bytes[8] = (bytes[8] & 0x3f) | 0x80;

  const hex = Array.from(bytes, (b) => b.toString(16).padStart(2, "0"));
  return `${hex.slice(0, 4).join("")}-${hex.slice(4, 6).join("")}-${hex.slice(6, 8).join("")}-${hex.slice(8, 10).join("")}-${hex.slice(10, 16).join("")}`;
}
