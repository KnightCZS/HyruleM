# Auditoria 0.5.0-preview.11

## Evidência entregue

O jogador informou que a prévia 10 não alterou o resultado. O `diagnostico.txt` identifica o launcher 10, conexão/presença, duas tentativas de maçã canceladas e encerramento da conexão de controle. Ele mistura histórico antigo com a sessão recente e não contém `hm_native_status`, `hm_local_state` ou `hm_items_status` desta sessão. Os espaços de runtime dos clientes estavam vazios. Não permite concluir que havia maçãs, qual etapa da criação falhou ou se os ossos foram recebidos. Os avisos de shader do histórico, por si só, não explicam os defeitos identificados no código.

O guia mestre foi lido como recomendações de arquitetura. Ele se baseia no que o pacote declarava; suas tabelas não confirmam que a implementação estivesse integrada ou visível no jogo. Não foram publicados os arquivos enviados, seus caminhos/IPs ou conteúdo privado do jogo.

## Falhas confirmadas e correções

1. `Bridge.run` lia a pose completa, mas depois copiava manualmente yaw, tracks e equipamentos para outra struct. O campo `Skeleton` ficava vazio em todo envio contínuo. O código agora publica o quadro capturado inteiro e acrescenta só velocidade/fase calculadas. A regressão de dois clientes falha no código anterior e passa após a correção; não é apenas um teste do codec.
2. Armadura procurada em tipo de pouch 2. O jogo distingue flechas (2), cabeça (4), tronco (5) e pernas (6). A leitura atual usa o tipo correto e confere o sufixo correspondente. O teste anterior também usava a premissa errada. Foi corrigido e ampliado para as três peças, flechas e itens não equipados; a fixture com tipo 5 falhou antes da correção.
3. Categorias do getter nativo são espada/escudo/arco; os slots da rede são espada/arco/escudo. A inversão fazia os validadores descartarem arco e escudo. Agora há uma conversão explícita e teste com os três atores nativos. A ordem também é usada pelo recurso quick-drop do runtime fixado.
4. No início de um intervalo, o codec selecionava o esqueleto antigo e sinalizava snap, enquanto o cliente mantinha a arma nova. Uma troca de nome de equipamento também podia conservar uma matriz antiga. A seleção visual agora mantém os dois no mesmo quadro. Testes exercitam alpha zero, meio intervalo, troca de arma e matriz degenerada.
5. Uma evidência de morte podia ser capturada depois de uma escrita de vida zero já armazenada no cache do launcher. O cache impedia sua confirmação e a fila nativa ficava pendente. Zero é reenviado no máximo uma vez por segundo por identidade, respeitando quatro comandos por atualização. Testes com sete e 25 entidades confirmam a confirmação tardia e a rotação do orçamento, sem limitar o atendimento às primeiras 16 identidades.
6. O diagnóstico textual do launcher não consultava os estados nativos ao vivo. A coleta agora roda em segundo plano, com timeout global e endpoint de cada slot, sem hash de bibliotecas grandes nem inspeção de memória privada de atores. O diagnóstico detalhado separado continua disponível e também procura a biblioteca em `extracted/`.

## Observabilidade e descartes

O runtime publica contagens distintas de ossos capturados, recebidos, aplicados no corpo e aplicados no total. O motivo da captura incompleta é explícito; matrizes inválidas continuam rejeitadas, sem publicar um quadro corrompido. Telemetria periódica fica no log do processo da sessão. O diagnóstico distingue captura, transporte e mapeamento do corpo, sem tratar contador de escrita como prova de renderização.

Descarte de maçã publica motivo, quantidade antes e depois, e conserva idempotência. Falta de maçã, layout de inventário inválido, falha de handle, criação/ativação recusada e retirada não confirmada não viram o mesmo erro genérico. Um teste passa pelo fluxo nativo com inventário vazio, confere motivo `apple-missing` e verifica que repetir o pedido não cria/debita outra vez. Isso não afirma que falta de maçã foi a causa das falhas do jogador.

## Compatibilidade e validação

Launcher/runtime: ABI 8; rede: protocolo 8. Gamelib permanece ABI 4, com os mesmos hooks, endereços e assinaturas da prévia 10. Uma biblioteca privada 10 válida pode continuar sendo usada quando o runtime aceita sua identidade; gerações anteriores seguem sujeitas à reconstrução normal. Nenhuma edição de `.id` é utilizada para contornar a verificação.

Testes executados: Go/race/vet; fluxo captura → bridge contínua → servidor TCP → cliente remoto → comando nativo, nos dois sentidos; seleção de quadros e confirmação tardia de mortes; diagnóstico leve por socket; testes C de equipamentos, matrizes, referências, fila, inventário e erros. Mantêm-se os testes anteriores de reserva, condição/modificadores e recuperação de capacidade. Aplicação do adaptador é idempotente; emissão dos hooks é compilada/executada. Resolução privada 1.9.0 encontra 191 sites, incluindo os 26 do adaptador e os auxiliares de inventário. Build/empacotamento/instalador Windows são conferidos no CI da release.

Não realizado: gameplay/GPU desta versão, medição de FPS, conferência da fase final do renderer e validação dos descartes em jogo real. Não anunciar ausência de todos os bugs nem visibilidade confirmada com base nos testes sintéticos.

## Objetivo ainda pendente

Vários Links completos exigem eventos de ação, colisão/dano, projéteis/runas, autoridade de objetos/IA e persistência. Não foram substituídos por uma simulação fictícia. O mundo experimental continua limitado a Hateno/90 m/32 identidades; árvores caídas, física, posição/IA dos monstros, baús e flags gerais não estão comprovados/implementados integralmente. O servidor não executa BOTW. A correção desta release remove defeitos reais da integração existente e mantém o restante explicitamente pendente.
