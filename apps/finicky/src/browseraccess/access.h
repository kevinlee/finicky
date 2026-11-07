#ifndef FINICKY_BROWSER_ACCESS_H
#define FINICKY_BROWSER_ACCESS_H

// The caller owns the returned error string. NULL means success.
char *FinickyRestoreBrowserAccess(void);
void FinickyRequestBrowserAccess(unsigned long long requestID);
extern void FinickyBrowserAccessFinished(unsigned long long requestID, char *message);

#endif
