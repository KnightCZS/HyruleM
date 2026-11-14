# Revisão do HyruleM — prévia 9

Revisão dos módulos escritos para launcher, preparação/sandbox/filtro, instâncias, transporte, ambiente, presença nativa, recursos visuais, diagnósticos e regras futuras Gen1. Base publicada: `5813c5119088df15373e7a09f18d37125556332d` (prévia 8). O objetivo foi encontrar falhas concretas e acrescentar regressões úteis, sem assumir que testes provam ausência de todos os bugs.

## Problemas encontrados e tratados

| Área | Problema | Correção / evidência |
|---|---|---|
| Preferências | Selecionar outro executável/instalar runtime reconstruía Config e perdia preferências do sandbox | Atualização apenas dos campos da instalação; save base, região e perfil permanecem |
| Clima/relógio | A limpeza descartava o estado mesmo se uma chamada de restauração falhasse | Retentativa preserva os campos pendentes e reconhece etapas já restauradas; teste de falha parcial e política manual |
| Equipamento | O prazo era contado desde a criação e podia falhar imediatamente por um acesso negado depois de longa atividade/pausa | Renovação na aplicação válida e na pausa; corpo continua independente de uma peça |
| Animações | Prefixos amplos podiam suprimir um Wait/Move especial que não fosse contínuo | Lista explícita de nomes de locomoção; ataques forçados continuam sequenciados |
| Interpolação | Com o buffer ainda curto, escolhia o snapshot futuro; retorno podia expor a slice interna de equipamento | Usa o primeiro estado até haver intervalo; devolve cópia; regressões de início/aliasing |
| Hospedagem | O mesmo campo de porta servia para ouvir e conectar, e ficava bloqueado ao hospedar | Campo separado para porta de hospedagem; validação de IP/porta mantida |
| Organização | A janela principal misturava configuração, jogo, sandbox e ferramentas numa lista longa | Páginas separadas, conexão prioritária, segundo jogo opcional; controles mantêm seus valores ao navegar |
| Pintura | Cantos dos botões podiam ficar claros; a janela multiplayer não usava o mesmo buffer de pintura | Preenchimento dos cantos e pintura em buffer nos dois menus; capturas Windows e verificação de recursos GDI |
| Estado antigo | Observações de ambiente/vida podiam vir de uma pose antiga mantida por heartbeat | Validação de pose recente no servidor |
| Materiais | Inferir coleta pelo desaparecimento confundiria descarregamento/destruição com aquisição | Evento explícito de coleta, reserva do servidor e recibo da quantidade local |
| Materiais | Repetir um pedido cujo ID o servidor já resolveu podia ser interpretado como troca de identidade | Resolução idempotente admite o ID original ausente e rejeita troca por outro ID explícito; regressão TCP completa |
| Materiais | Uma leitura ambígua após concessão poderia liberar o objeto e duplicar a unidade | Mantém a reserva pendente; não confirma falha nem repete a concessão |
| Materiais | Uma réplica ainda em criação poderia reaparecer na varredura como novo drop | Identificação pelo próprio handle assíncrono e filtro das réplicas; nenhuma heurística de sumiço para coleta |

## Itens revisados que permanecem deliberadamente restritos

- A conexão de controle continua local, com limites por comando, enquadramento por linha e descarte do socket em resposta inválida. A moldura visual completa usa um comando atômico; materiais têm limite próprio de 511 bytes.
- As escritas de ator ficam no callback pós-jobs. A captura AS e o hook de coleta não executam funções guest, não aguardam rede e preservam a CPU.
- Referências retidas verificam geração, identidade e acesso nativo. Limpeza negada mantém a referência para outra tentativa; não usa ponteiro bruto como contingência.
- O filtro/sandbox e o overlay são preparados fora da UI e publicados com os jogos fechados. Cada cliente tem save/config/cache/porta próprios; os assets extraídos são vinculados, sem alteração na origem.
- Recursos visuais continuam gerados localmente. O conjunto privado anterior de 602 prefabs foi validado estruturalmente; não há recursos do jogo no repositório/pacote.
- O protocolo Gen1 separado continua uma estrutura de desenvolvimento. Suas regras de combate/IA não foram anunciadas como funcionando só por existirem no servidor. As capacidades completas permanecem bloqueadas.
- O servidor de presença não é uma cópia do motor do jogo. Esta etapa de mundo é um registro pequeno de materiais, com autoridade e reservas, sem simulação de toda Hyrule.

## Validação e limites da conclusão

- Testes Go de protocolo/servidor/bridge, incluindo detector de corrida e vet dos pacotes internos; dois clientes TCP reais em memória sintética de jogo.
- Testes C de vida útil, CPU, barreira dos jobs, roupa/armas, coleta adiada, quantidade exata, repetição e resultado ambíguo.
- Fragmentos do callback de mundo e do hook de coleta compilados/executados, preservando também a instrução substituída.
- Dezenove padrões únicos conferidos na cópia privada do módulo 1.9.0. Os offsets não foram importados da versão 1.5.0 de decompilação.
- UI Windows exercitada com preparação que falha de propósito, hospedagem, navegação, dois clientes, mudança de DPI e capturas de cada página. Os logs completos continuam disponíveis quando uma mensagem visual é resumida.
- Compilação/instalador Windows e integridade do ZIP registrados no CI da versão.

**Ainda não comprovado nesta revisão:** desenho dos equipamentos na GPU, execução do fluxo de coleta no jogo real, efeitos especiais/fanfarras, física compartilhada, mapas interiores, recuperação automática de reserva incerta e persistência de itens ao reiniciar servidor. Por isso o experimento de materiais vem desligado inicialmente e se limita aos cinco materiais e ao raio de Hateno descritos no README. O limite de 32 identidades inclui tombstones, evitando crescimento sem limite e ressurreição por snapshot atrasado.

Não é necessário testar agora para usar o launcher reorganizado ou manter a presença. A ampliação do experimento depende de uma validação futura em dois jogos reais.

O vet adicional do pacote Win32 aponta as conversões de `lParam` para estruturas de mensagem do Windows (`unsafe.Pointer`), já usadas pelos callbacks. Elas foram conferidas como ponteiros fornecidos pelo sistema; não foram tratadas como uma prova de acesso de memória inválido nem removidas com casts artificiais. A regressão Windows exercita esses callbacks, incluindo DPI/desenho. Esse comando adicional não é reportado como passando sem avisos.
