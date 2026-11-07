#ifndef FINICKY_PICKER_H
#define FINICKY_PICKER_H

void FinickyShowPicker(unsigned long long requestID, const char *requestJSON);
extern void FinickyPickerDidSelect(unsigned long long requestID, int index, unsigned int flags);

#endif
