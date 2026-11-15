    // Reproduce a real failed apple operation through its native queue. The
    // player gets an explicit reason, and retry cannot debit or create twice.
    ground_reset();r.failed=false;ground_open(6,material_open,&r);
    ST64(head,head);ST32(creator+0x48,0);ground_tick(&cpu,owner,0x7000001000);
    ST64(ground_pouch_item+0x30,ground_pouch_item+0x80);strcpy((char *)(uintptr_t)(ground_pouch_item+0x80),"Item_Fruit_A");ST32(ground_pouch_item+0x18,7);ST8(ground_pouch_item+0x25,1);ST32(ground_pouch_item+0x20,0);
    empty_inventory=true;drop_state[3]="3";drop_state[12]="1";drop_state[15]="0";drop_state[17]=ground_origin;ground_update(22,drop_state,&r);assert(!r.failed);
    drop_command[3]="3";drop_command[4]="1";ground_drop_command(5,drop_command,&r);assert(!r.failed);before_create=creates;debit_before=inventory_debits;
    ground_tick(&cpu,owner,0x7000001000);ground_tick(&cpu,owner,0x7000001000);
    assert(ground_drop_last_result==-1&&!strcmp(ground_drop_error,"apple-missing")&&ground_drop_before==0&&ground_drop_after==-1&&inventory_debits==debit_before);
    ground_drop_command(5,drop_command,&r);ground_tick(&cpu,owner,0x7000001000);assert(creates==before_create+1&&inventory_debits==debit_before);
    empty_inventory=false;ground_reset();assert(!ground_drop_error[0]&&ground_drop_before==-1&&ground_drop_after==-1);
    puts("preview 11: real armor types, native sword/shield/bow categories, pose diagnostics and apple failure without repeated debit passed");
