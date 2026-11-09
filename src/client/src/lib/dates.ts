// The stored dates arrive as PocketBase timestamps. Everything this dashboard
// dates is dated by the day and not by the instant: a chief editor sets a
// submission aside on a day, and a review deadline falls on a day. One helper,
// so two screens showing the same stored value cannot format it differently.
export const day = (value: string) => value.slice(0, 10);
