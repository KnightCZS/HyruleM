# Prévia 8: ambiente e aparência

O retorno do jogador confirma visibilidade e movimento com a prévia 7. A etapa seguinte mantém essa integração e acrescenta as etapas 1–4 solicitadas; ainda não equivale a mundo/IA compartilhados.

| Etapa | Implementação atual | Limite |
|---|---|---|
| 1. Horário e clima | Servidor escolhe anfitrião/época; bridge lê e aplica comandos existentes; sucessão, entrada tardia e restauração de políticas | Clima global do host, sem dia/Blood Moon/eventos |
| 2. Módulo nativo | C no runtime, fila local e callback após os jobs | Alvo estrito Switch 1.9.0 |
| 3. Marionete | DemoNPC derivado localmente por overlay, referências nativas e criação assíncrona | Sem controlador Player/IA/física de combate |
| 4. Animação/equipamento | AS de dois canais e slots, seis peças visuais, matrizes relativas, suavização | Sem fase exata de clipe, tinturas, máscaras especiais e bainhas |

ABI 5/protocolo 5 exigem atualizar runtime, launcher e servidor. Sites, emissor e headers usados pela biblioteca gerada permanecem os da prévia 7. O overlay ganha nova identidade para obrigar regeneração com os jogos fechados.

## O relatório do Milk Bar e as diferenças

O README e o código de Player/LocalInstance mostram servidor C#, DLL C++, WPF e pacote BNP. Há pedidos explícitos de criação na fila: não é correto concluir apenas que a DLL espera passivamente um ator criado por dados. A ideia aproveitável é separar ciclo de vida, pose, aparência e mundo; nenhum código ou BNP foi copiado.

Aqui a criação nativa já foi localizada e confirmada em jogo. Trocar esse caminho comprovado por um spawn indireto de mapa acrescentaria outra dependência. Mantemos o corpo DemoNPC e usamos prefabs derivados dos dados locais para as peças visuais. ActorInfo define o arquivo/unidade real de cada modelo; armas usam template rígido simples e armaduras mantêm máscaras parciais do esqueleto.

O projeto zeldaret/botw identifica Switch 1.5.0. Nomes podem orientar pesquisa conceitual, mas não fornecem offsets reutilizáveis na 1.9.0; não usamos código decompilado. Os getters/transformações foram conferidos no módulo fornecido privadamente.

Os testes sintéticos comprovam enfileiramento, referências, preservação de CPU e valores aplicados. A leitura independente dos packs comprova estrutura e recursos. Somente o teste com dois jogos pode confirmar aparência, alinhamento e animação na GPU.

## Histórico: correção da issue #6

# Atualização após as tarefas dos atores — issue #6

Os relatórios da prévia 6 confirmaram criação/visibilidade estática, mas nenhum movimento ou pedido AS: o resolver nativo retornou acesso negado durante toda a janela. A integração anterior dentro do job de dano do jogador podia atingir a proteção de acesso concorrente a outros atores. A correção atual substitui esse ponto pelo retorno da espera das tarefas dos atores, após o reset nativo da fase de acesso. A validação em execução privada com memória sintética reproduziu negação de um ator ativo no worker, espera de dois workers e resolução permitida após o reset. Não constitui teste gráfico.

Na prévia 7, ABI 4 exigiu recompilar a biblioteca com o gerador novo; o protocolo era 4. O adaptador não desliga os checks nativos, não resolve por ponteiro bruto e não usa a thread do renderer. A captura local de AS mantém seu hook separado, somente de leitura.

## Histórico da integração anterior

# Comparação e correção da presença invisível

O relato do teste anterior confirmou conexão sem visibilidade do outro Link. A inspeção do módulo encontrou um erro suficiente para bloquear toda a criação: o parâmetro da rotina interceptada é o controlador de dano e o ator está em `this + 0x18`. O teste anterior usava diretamente um ator como parâmetro e não reproduzia a chamada do jogo. A correção e a regressão usam agora o controlador com seu dono.

| Referência primária | Mecânica estudada | Aplicação no HyruleM |
|---|---|---|
| [Milk Bar: Player](https://github.com/MilkBarModding/MilkBarLauncher/blob/main/DLL/InjectDLL/Player.cpp) e [LocalInstance](https://github.com/MilkBarModding/MilkBarLauncher/blob/main/DLL/InjectDLL/LocalInstance.h) | Ator próprio, configuração e ciclo de vida | A conexão é separada da confirmação de atualização, criação, modelo e animação |
| [Milk Bar: EnemyAccess](https://github.com/MilkBarModding/MilkBarLauncher/blob/main/DLL/InjectDLL/EnemyAccess.h) e [Enemy](https://github.com/MilkBarModding/MilkBarLauncher/blob/main/DLL/InjectDLL/Enemy.h) | Identidade de mapa e menor vida conhecida | Convergência de vida sem copiar a vida dos jogadores |
| [PrivCord: ghost Links](https://github.com/Blacknapoleon99/privcord-botw-coop/blob/main/docs/the-long-bug.md) e [arquitetura](https://github.com/Blacknapoleon99/privcord-botw-coop/blob/main/docs/how-it-works.md) | Mensagens TCP fragmentadas e distinção entre pose, criação e IA | Enquadramento por mensagem e testes de fragmentação; autoridade da IA permanece separada |
| [BreathM](https://github.com/msork/BreathM/blob/main/README.md) | O README Alpha 0.7 declara networking e presença; gameplay ainda não iniciado | Não fornece uma implementação pronta de atores ou mundo para portar |

Cemu usa Wii U/PowerPC, enquanto nosso runtime usa Switch 1.9.0/AArch64. Endereços e estruturas não são intercambiáveis. A implementação do HyruleM é original; nenhum código ou recurso do Milk Bar, cuja licença reserva todos os direitos, foi incorporado.

Vida compartilhada é uma primeira camada. IA por proximidade, coleta exclusiva, dano entre jogadores e objetos exigem callbacks adicionais. As regras Gen1 no servidor não tornam essas funcionalidades prontas no jogo.

## Retorno da issue #4

O relatório acessível informa `preview_abi: 1`, `hm_native_status` sem contadores de atualização e comando `hm_world_status` desconhecido. O SHA-256 do executável informado difere do binário publicado na prévia 2. Os outros dois relatórios tiveram conexão de controle recusada. Esses dados demonstram uso do adaptador antigo; não demonstram que a correção ABI 2 tenha executado.

A prévia 3 trata a instalação como parte do fluxo: atalho confere e instala no destino configurado, modo do EXE é selecionado pelo pacote, launcher exige identidade correspondente e a mensagem de falha sobrevive à limpeza da conexão. O instalador lê JSON UTF-8 explicitamente no Windows PowerShell. Não muda a implementação do ator nem anuncia confirmação de renderização.
