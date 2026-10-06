SELECT TOP {{PRODUCTION_COUNT}} 
	Uretim.Id AS id,
	Uretim.BasTarih AS production_dt_start,
	UretimSonuc.BitisTarihi AS production_dt_end,
	UretimSonuc.Miktar AS concrete_quant,
	Recete.ReceteAdi AS concrete_descr,
	Uretim.Olusturan AS user_descr,
	Uretim.AracPlaka AS vehicle_descr,
	NULL AS correction_id,
	NULL AS correction_dt_end,  
	
	'Инертные1' AS mat1_descr,
	UretimSonuc.Agrega1 AS mat1_quant,
	UretimSonuc.Agrega1Istenen AS mat1_quant_req,
	CAST(0 AS BIT) AS mat1_cement,
	NULL AS mat1_quant_corrected,
	
	'Инертные2' AS mat2_descr,
	UretimSonuc.Agrega2 AS mat2_quant,
	UretimSonuc.Agrega2Istenen AS mat2_quant_req,
	CAST(0 AS BIT) AS mat2_cement,
	NULL AS mat2_quant_corrected,
	
	'Инертные3' AS mat3_descr,
	UretimSonuc.Agrega3 AS mat3_quant,
	UretimSonuc.Agrega3Istenen AS mat3_quant_req,
	CAST(0 AS BIT) AS mat3_cement,
	NULL AS mat3_quant_corrected,

	'Инертные4' AS mat4_descr,
	UretimSonuc.Agrega4 AS mat4_quant,
	UretimSonuc.Agrega4Istenen AS mat4_quant_req,
	CAST(0 AS BIT) AS mat4_cement,
	NULL AS mat4_quant_corrected,

	'Инертные5' AS mat5_descr,
	UretimSonuc.Agrega5 AS mat5_quant,
	UretimSonuc.Agrega5Istenen AS mat5_quant_req,
	CAST(0 AS BIT) AS mat5_cement,
	NULL AS mat5_quant_corrected,

	'Инертные6' AS mat6_descr,
	UretimSonuc.Agrega6 AS mat6_quant,
	UretimSonuc.Agrega6Istenen AS mat6_quant_req,
	CAST(0 AS BIT) AS mat6_cement,
	CAST(0 AS BIT) AS mat6_quant_corrected,
					
	'Хим. добавки1' AS mat7_descr,
	UretimSonuc.Katki1 AS mat7_quant,
	UretimSonuc.Katki1Istenen AS mat7_quant_req,
	CAST(0 AS BIT) AS mat7_cement,
	NULL AS mat7_quant_corrected,
	
	'Хим. добавки2' AS mat8_descr,
	UretimSonuc.Katki2 AS mat8_quant,
	UretimSonuc.Katki2Istenen AS mat8_quant_req,
	CAST(0 AS BIT) AS mat8_cement,
	NULL AS mat8_quant_corrected,
	
	'Хим. добавки3' AS mat9_descr,
	UretimSonuc.Katki3 AS mat9_quant,
	UretimSonuc.Katki3Istenen AS mat9_quant_req,
	CAST(0 AS BIT) AS mat9_cement,
	NULL AS mat9_quant_corrected,

	'Хим. добавки4' AS mat10_descr,
	UretimSonuc.Katki4 AS mat10_quant,
	UretimSonuc.Katki4Istenen AS mat10_quant_req,
	CAST(0 AS BIT) AS mat10_cement,
	NULL AS mat10_quant_corrected,

	'Вода1' AS mat11_descr,
	UretimSonuc.Su1 AS mat11_quant,
	UretimSonuc.Su1Istenen AS mat11_quant_req,
	CAST(0 AS BIT) AS mat11_cement,
	NULL AS mat11_quant_corrected,

	'Вода2' AS mat12_descr,
	UretimSonuc.Su2 AS mat12_quant,
	UretimSonuc.Su2Istenen AS mat12_quant_req,
	CAST(0 AS BIT) AS mat12_cement,
	NULL AS mat12_quant_corrected,
	
	'Цемент1' AS mat13_descr,
	UretimSonuc.Cimento1 AS mat13_quant,
	UretimSonuc.Cimento1Istenen AS mat13_quant_req,
	CAST(1 AS BIT) AS mat13_cement,
	NULL AS mat13_quant_corrected,

	'Цемент2' AS mat14_descr,
	UretimSonuc.Cimento2 AS mat14_quant,
	UretimSonuc.Cimento2Istenen AS mat14_quant_req,
	CAST(1 AS BIT) AS mat14_cement,
	NULL AS mat14_quant_corrected,
	
	'Цемент3' AS mat15_descr,
	UretimSonuc.Cimento3 AS mat15_quant,
	UretimSonuc.Cimento3Istenen AS mat15_quant_req,
	CAST(1 AS BIT) AS mat15_cement,
	NULL AS mat15_quant_corrected,

	'Цемент4' AS mat16_descr,
	UretimSonuc.Cimento4 AS mat16_quant,
	UretimSonuc.Cimento4Istenen AS mat16_quant_req,
	CAST(1 AS BIT) AS mat16_cement,
	NULL AS mat16_quant_corrected
	
FROM Uretim
LEFT JOIN UretimSonuc ON UretimSonuc.UretimId=Uretim.id
LEFT JOIN Recete ON Recete.Id=Uretim.ReceteId
WHERE 
	Uretim.Id {{SIGN}} {{PRODUCTION_ID}}
ORDER BY Uretim.Id

